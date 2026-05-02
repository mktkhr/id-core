# バックエンドテスト規約

> 最終更新: 2026-05-02 (M0.3: DB を要するテスト / migrate 整合 / `/health/ready` テストパターン追加)

## id-core (Go) のテスト

### 標準パターン

- パッケージレイアウト: 外部テストパッケージ (`<pkg>_test`) を基本とし、内部 API を検証する場合のみ内部テスト (`<pkg>` 同名) を併用する
- テストランナ: `go test -race ./...` (`make test`)。カバレッジは `make test-cover`
- HTTP ハンドラ: `httptest.NewRequest` + `mux.ServeHTTP(rec, req)` で組み立てる (M0.1 から踏襲)。middleware を含めた検証は `server.New` の handler 全体を `httptest.NewServer` で起動する
- 並列化: `t.Parallel()` を基本にする。ただし `t.Setenv` を使うテストは Go の仕様で `t.Parallel` と**併用不可**のため直列実行する
- 環境変数の解除は `t.Setenv("KEY", "")` で空文字を設定し、Load 側で空文字をデフォルト扱いにする (Unsetenv 不可)

### テーブル駆動テスト

deny-list 系 (redact) や境界値検証は **テーブル駆動テスト**を用いる。

```go
func TestIsFieldKeyToRedact(t *testing.T) {
    cases := []struct {
        key  string
        want bool
    }{
        {"password", true},
        {"PASSWORD", true},      // case-insensitive
        {"my_password", false},  // 部分一致は対象外
        {"client_secret", true},
    }
    for _, tc := range cases {
        t.Run(tc.key, func(t *testing.T) {
            if got := logger.IsFieldKeyToRedact(tc.key); got != tc.want {
                t.Errorf("IsFieldKeyToRedact(%q) = %v, want %v", tc.key, got, tc.want)
            }
        })
    }
}
```

### ログ buffer での検証パターン

ロガーの出力を検証する場合は `bytes.Buffer` を `logger.New` の writer に渡し、JSON Lines を 1 行ずつ `json.Unmarshal` で `map[string]any` にデコードしてフィールド存在 + 型を検証する。`encoding/json` は数値を `float64` にデコードする点に注意。

```go
var buf bytes.Buffer
l := logger.New(logger.FormatJSON, &buf)
ctx := logger.WithRequestID(context.Background(), "test-id")
l.Info(ctx, "access", "method", "GET", "path", "/", "status", 200, "duration_ms", 1.0)

out := strings.TrimSpace(buf.String())
var m map[string]any
if err := json.Unmarshal([]byte(out), &m); err != nil {
    t.Fatalf("Unmarshal: %v (out=%q)", err, out)
}
if _, ok := m["request_id"].(string); !ok {
    t.Errorf("request_id missing or not string, record=%v", m)
}
```

### `log.Fatal*` ガード (Makefile lint)

`make lint` は `go vet` に加えて、`core/` 配下の非テスト `.go` ファイルに `log.Fatal` / `log.Fatalf` / `log.Fatalln` の呼び出しが新規追加されていないかを `grep` で検査する (F-12)。違反時は明示エラーで lint failure。回避策は `logger.Error(ctx, msg, err)` + `os.Exit(1)` を使うこと。

### DB を要するテスト (M0.3)

DB 接続を必要とするテストは `//go:build integration` ビルドタグで分離する。これにより `make test` (DB 不要) と `make test-integration` (DB 必要) を明確に区別できる。

```go
//go:build integration

package db_test

import (
    "testing"
    "github.com/mktkhr/id-core/core/internal/testutil/dbtest"
)

func TestOpen_Success_Integration(t *testing.T) {
    ctx, pool := dbtest.NewPool(t)  // CI=fatal / local=skip 自動分岐
    tx := dbtest.BeginTx(t, ctx, pool)
    defer dbtest.RollbackTx(t, ctx, tx)
    // ...
}
```

#### tx-rollback ハイブリッドパターン (Q8 / F-17)

各テストは TX を開始し、`defer Rollback` で必ず巻き戻す。これによりテスト並列実行 (T-81) でも互いに不可視 + テスト失敗後に残留 state なし (T-82) を保証する。

#### `dbtest` ヘルパー (`core/internal/testutil/dbtest`)

| API                      | 用途                                                                    |
| ------------------------ | ----------------------------------------------------------------------- |
| `NewPool(t)`             | `*pgxpool.Pool` 取得 + 初回 Ping。CI で fatal、ローカルで skip 自動分岐 |
| `BeginTx(t, ctx, pool)`  | `pool.Begin(ctx)` のラッパ。失敗時 `t.Fatal`                            |
| `RollbackTx(t, ctx, tx)` | `tx.Rollback(ctx)` のラッパ。`pgx.ErrTxClosed` は無視 (Commit 済を許容) |

#### `make test-integration` ターゲット

`core/Makefile` から実行する:

```
TEST_DATABASE_URL=... TEST_DB_REQUIRED=1 \
go test -p 1 -race -v -tags integration ./...
```

- `-p 1`: package 単位順次実行 (将来 truncate 等グローバル state 操作が入った場合の安全性確保)
- `-race`: data race 検出
- `-tags integration`: build tag で除外されたテストファイルを有効化
- `TEST_DB_REQUIRED=1`: 接続失敗を skip ではなく fail に (CI 必須)

#### CI / ローカルの挙動分離

| 環境     | `TEST_DB_REQUIRED` | DB 接続失敗時の挙動                                |
| -------- | ------------------ | -------------------------------------------------- |
| CI       | `1` (必須)         | `t.Fatal` (テスト失敗)                             |
| ローカル | 未設定             | `t.Skip` (DB を立てずにユニットテストのみ実行可能) |

## migrate 整合テストパターン (M0.3)

`core/internal/dbmigrate/migrate_integration_test.go` で F-14 double-roundtrip を検証する。

### F-14 double-roundtrip 3 条件

| #   | 条件                                  | 検証内容                                                                         |
| --- | ------------------------------------- | -------------------------------------------------------------------------------- |
| a   | object 出現 / 消失                    | Up 後に `schema_smoke` 存在、Down 後に削除済                                     |
| b   | `schema_migrations` が initial と一致 | double-roundtrip (Up→Down→Up→Down) 完了後に `AssertClean` が `nil` (clean state) |
| c   | 全工程で no-error                     | 各 Up / Down 呼び出しが nil error                                                |

```go
//go:build integration

func TestDoubleRoundTrip_F14(t *testing.T) {
    ctx, _ := dbtest.NewPool(t)
    src := migrationsURL(t)
    dsn := dbtest.DatabaseURL()
    l := newTestLogger()

    for _, fn := range []func() error{
        func() error { return dbmigrate.RunUp(ctx, dsn, src, l) },
        func() error { return dbmigrate.RunDown(ctx, dsn, src, l) },
        func() error { return dbmigrate.RunUp(ctx, dsn, src, l) },
        func() error { return dbmigrate.RunDown(ctx, dsn, src, l) },
    } {
        if err := fn(); err != nil { t.Fatalf("...") }
    }
    if err := dbmigrate.AssertClean(ctx, dsn, src, l); err != nil {
        t.Errorf("AssertClean: %v", err)
    }
}
```

### dirty 検出経路 (T-89)

`schema_migrations.dirty=true` を SQL で直接立て、`AssertClean` が `errors.Is(err, ErrDirty)` で判定可能なエラーを返すことを検証する。

## `/health/ready` の DB チェックテストパターン (M0.3)

ユニットテスト (M0.3 P3 で実装済) は `pingPool` interface の stub で `pool.Ping` をモックする。統合テスト (M0.3 P4) は `dbtest.NewPool` で実機 DB に対して実行。

| 観点             | 単体テスト (T-94〜T-98)                                 | 統合テスト (T-94 強化版) |
| ---------------- | ------------------------------------------------------- | ------------------------ |
| Ping 成功 → 200  | stub に `nil` を返させて検証                            | 実機 pool で 200 確認    |
| Ping 失敗 → 503  | stub に sentinel error を返させて検証                   | -                        |
| timeout (2s)     | stub に 3s delay を入れて検証                           | -                        |
| F-7 公開粒度下限 | 503 body に DB 詳細含まないことを assert                | -                        |
| F-10 redact      | 503 時の内部ログに DSN / password 含まないことを assert | -                        |

統合テストは「成功経路の冒煙」のみで十分。失敗経路は単体テストで網羅 (DB 停止状態を CI で再現するのは複雑なため)。

### ログスキーマ契約テスト (F-16)

`core/internal/logger/contract_test.go` がログスキーマの破壊的変更を検知する契約テストを提供する。検証対象は 2 系統に分かれる:

| 系統             | 必須フィールド                                                                                               |
| ---------------- | ------------------------------------------------------------------------------------------------------------ |
| HTTP 経路 (a)    | `time` / `level` / `msg` / `request_id` / `method` / `path` / `status` / `duration_ms` (型: string + number) |
| 非 HTTP 経路 (b) | `time` / `level` / `msg` / `event_id` (型: string)                                                           |

方針:

- フィールドの**追加は許容** (前方互換)。既存テストは追加された属性を無視する
- フィールドの**削除・型変更はテスト失敗**として扱う (破壊的変更検知)
- 値の正確性 (例: `request_id` が UUID v7 か) は契約テストの対象外。別の単体テスト (`request_id` middleware 等) が個別に検証する

## go-react バックエンド (Go) のテスト

TBD

## kotlin-nextjs バックエンド (Spring Boot / Kotlin) のテスト

TBD

## OIDC フローの統合テスト

TBD — id-core の OIDC OP として、上流 IdP モック / 下流 RP モックを使った end-to-end の OIDC フロー検証方針。
