# バックエンド実装パターン (id-core / Go)

> 最終更新: 2026-05-02 (M0.3: DB 接続 / マイグレーション運用 / 統合テスト / context ID 伝播 パターン追加)

## アーキテクチャパターン

M0.1 段階では最小骨格 (cmd + internal の単純構造)。
M1.x の OIDC エンドポイント実装着手時に **Package by Feature + クリーンアーキテクチャ** を導入する (`backend-architecture` スキル参照)。

## ServeMux ハンドラ登録パターン (Go 1.22+)

メソッド指定パターンを使い、未対応メソッドは `ServeMux` の標準挙動に任せる (405 + `Allow` ヘッダ)。

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /health", health.NewHandler(l).ServeHTTP)
// 他メソッドは自動で 405 + Allow: GET, HEAD
```

## 設定読み込み + main 分離パターン

`config` 層は `error` を返し、`main` で `logger.Error` + `os.Exit(1)` する責務分担。テスト容易性を担保し、`log.Fatal*` は使用しない (Makefile の lint で検査)。

```go
// internal/config/config.go
func Load() (*Config, error) {
    // バリデーション失敗時は error を返す (log.Fatal を直接呼ばない)
}

// cmd/core/main.go
func main() {
    ctx := logger.WithEventID(context.Background(), newEventID())
    l, err := logger.Default()
    if err != nil {
        // logger 初期化前の起動失敗は stderr に直書きする (logger が無い)。
        fmt.Fprintf(os.Stderr, "logger 初期化失敗: %v\n", err)
        os.Exit(1)
    }
    cfg, err := config.Load()
    if err != nil {
        l.Error(ctx, "設定の読み込みに失敗しました", err)
        os.Exit(1)
    }
    srv := server.New(cfg, l)
    if err := srv.ListenAndServe(); err != nil {
        l.Error(ctx, "サーバーの実行に失敗しました", err)
        os.Exit(1)
    }
}
```

## middleware チェーンパターン (D1 順序)

外側から `request_id` → `access_log` → `recover` → `handler` の順に wrap する。`server.New` が組み立てを集約し、`cmd/main` 側はチェーン構築を意識しない。

```go
// internal/server/server.go (抜粋)
func New(cfg *config.Config, l *logger.Logger) *http.Server {
    mux := http.NewServeMux()
    mux.HandleFunc("GET /health", health.NewHandler(l).ServeHTTP)

    // 内側から外側へ順に wrap (実行順は外側から内側)。
    handler := middleware.Recover(l, mux)
    handler = middleware.AccessLog(l, handler)
    handler = middleware.RequestID(handler)

    return &http.Server{Addr: cfg.Addr(), Handler: handler}
}
```

D1 順序の根拠は `docs/context/backend/conventions.md` の「middleware 構成」節を参照。

## context への ID 付与パターン

`logger` パッケージが提供する `WithRequestID` / `WithEventID` で派生 context を作り、後段に渡す。Domain 層は context から取り出すのみ (ロガーへの直呼び出しは禁止 / F-14)。

```go
// HTTP 経路: middleware が WithRequestID 済み ctx を handler に渡す。
func handler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    id := logger.RequestIDFrom(ctx) // middleware が必ず設定済み (空文字を防御的に許容)
    _ = id
}

// 非 HTTP 経路: 起動・ジョブが WithEventID 済み ctx を後段に渡す。
func startup() {
    ctx := logger.WithEventID(context.Background(), newEventID())
    bootstrap(ctx)
}

// Domain 層 (将来導入): context から取り出すのみ、ロガー直呼び出し禁止。
func (s *AccountService) Create(ctx context.Context, ...) error {
    // ログ出力は presentation / application 層に集約する。
    return s.repo.Insert(ctx, ...)
}
```

## redact パターン (deny-list)

ログ出力前に redact deny-list (Q8 完全一覧) のキーを `[REDACTED]` 固定値へ置換する。実装は `core/internal/logger/redact.go` に集約し、二重管理を禁止する。

```go
// internal/logger/redact.go (抜粋)
const RedactedValue = "[REDACTED]"

// case-insensitive かつ完全一致でキーを判定 (部分一致は誤検知防止のため禁止)。
func IsFieldKeyToRedact(key string) bool { /* ... */ }

// http.Header / map[string]any を再帰走査して deny-list キーを置換した *コピー* を返す。
// 元のオブジェクトは変更しない (immutable)。
func RedactHeaders(h http.Header) http.Header { /* ... */ }
func RedactMap(m map[string]any) map[string]any { /* ... */ }
```

クエリ文字列・form パラメータ等の deny-list 適用面は `IsFieldKeyToRedact` を呼び出して deny-list を再利用する (例: `middleware/access_log.go` の `redactQueryString`)。

## エラーハンドリング

`core/internal/apperror/` パッケージで `CodedError` を生成し、`apperror.WriteJSON` で HTTP レスポンスにシリアライズする (M0.2 で導入。M0.1 暫定の `fmt.Errorf` + `%w` のみのパターンを置き換える)。

```go
// 生成
err := apperror.
    New("INVALID_PARAMETER", "ポートは 1〜65535 の整数で指定してください").
    WithDetails(map[string]any{"field": "CORE_PORT", "received": "0"}).
    Wrap(cause) // 元エラーを error chain として保持 (errors.Is / errors.As 対応)

// 内部ログ用文字列化 (公開しない)
log.Error(ctx, "validation failed", err)

// HTTP レスポンスへの書き出し (presentation 層)
apperror.WriteJSON(w, http.StatusBadRequest, err, requestID)
```

panic 時は `recover` middleware が `INTERNAL_ERROR` 固定コードを書き出す (`apperror.CodeInternalError`)。

## ログ出力失敗時のフォールバックパターン

primary writer (stdout 等) の書き込み失敗時に stderr にフォールバックし、それも失敗したら atomic drop counter を増分する。`Logger.log` の呼び出し元にはエラーを返さない (リクエスト処理を止めない / Q9)。

```go
// internal/logger/fallback.go (抜粋)
type FallbackWriter struct {
    primary  io.Writer
    fallback io.Writer
    drops    atomic.Int64
}

func (w *FallbackWriter) Write(p []byte) (int, error) {
    pn, perr := w.primary.Write(p)
    if perr == nil {
        return pn, nil
    }
    // 部分書き込み: primary が pn バイト書いた後にエラー → 残りを fallback に。
    fn, ferr := w.fallback.Write(p[pn:])
    if ferr == nil && fn == len(p)-pn {
        return pn + fn, nil
    }
    w.drops.Add(1)
    return len(p), nil // bytes 数は呼び出し側ループ条件のため正の値を返す
}

func (w *FallbackWriter) DropCount() int64 { return w.drops.Load() }
```

## httptest ベースのハンドラテストパターン

ServeMux ごと組み立てて `mux.ServeHTTP(rec, req)` を呼ぶ。これによりルーティング (405 / Allow) も検証できる。middleware を含めた検証が必要な場合は `server.New` で組み立てた handler 全体を `httptest.NewServer` で起動する。

```go
func TestHandler_GET_Success(t *testing.T) {
    mux := http.NewServeMux()
    mux.HandleFunc("GET /health", health.NewHandler(testLogger(t)).ServeHTTP)

    req := httptest.NewRequest(http.MethodGet, "/health", nil)
    rec := httptest.NewRecorder()
    mux.ServeHTTP(rec, req)

    res := rec.Result()
    defer res.Body.Close()

    if res.StatusCode != http.StatusOK {
        t.Errorf("status = %d, want %d", res.StatusCode, http.StatusOK)
    }
}
```

## 環境変数を扱うテストパターン (`t.Setenv`)

- `t.Setenv` と `t.Parallel` は**併用不可** (Go の仕様)
- 環境変数を使うテストは直列実行する
- 未設定状態は `t.Setenv("KEY", "")` で空文字を設定し、Load 側で空文字をデフォルト扱いにする (Unsetenv 不可のため)

```go
func TestLoad_Default(t *testing.T) {
    t.Setenv("CORE_PORT", "")
    cfg, err := config.Load()
    // ... デフォルト値検証
}
```

## OIDC クライアント (上流 IdP 委譲) パターン

TBD (M4.x で確定)

## OIDC OP (下流プロダクト向け) パターン

TBD (M1.x で確定)

## 認証セッション管理パターン

TBD

## アカウントリンクパターン

TBD (M4.3 / M5.3 で確定)

## 電話番号 / SNS 認証パターン

TBD (M5.x で確定)

## DI / 依存注入

M0.2 では `server.New(cfg, l)` にコンストラクタで設定 + ロガーを渡す。
M0.3 で `pool *pgxpool.Pool` を追加し `server.New(cfg, l, pool)` に変更。
M1.x で UseCase / Repository を導入する際にコンストラクタ DI を `internal/server/router.go` に集約する。

## DB 接続パターン (M0.3 追加)

`core/internal/db` で `pgxpool.Pool` を生成し、起動時に Ping で接続性を検証する。
DSN 組み立ては `url.UserPassword` で userinfo 部分の特殊文字 (`@` `:` `/` `?` `#` `%` 空白) を安全にエスケープする。

```go
// internal/db/dsn.go (抜粋)
func BuildDSN(_ context.Context, cfg *config.DatabaseConfig) string {
    q := url.Values{}
    q.Set("sslmode", cfg.SSLMode)
    u := url.URL{
        Scheme:   "postgres",
        User:     url.UserPassword(cfg.User, cfg.Password),
        Host:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
        Path:     "/" + cfg.DBName,
        RawQuery: q.Encode(),
    }
    return u.String()
}

// internal/db/db.go (抜粋)
func Open(ctx context.Context, cfg *config.DatabaseConfig, l *logger.Logger) (*pgxpool.Pool, error) {
    dsn := BuildDSN(ctx, cfg)
    poolCfg, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        l.Error(ctx, "DB DSN の parse に失敗しました", err, "params", SafeRepr(ctx, cfg))
        return nil, err
    }
    poolCfg.MaxConns = cfg.MaxConns
    poolCfg.MinConns = cfg.MinConns
    // ... 他のプール設定
    pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
    if err != nil {
        l.Error(ctx, "DB 接続プールの生成に失敗しました", err, "params", SafeRepr(ctx, cfg))
        return nil, err
    }
    if err := pool.Ping(ctx); err != nil {
        l.Error(ctx, "DB 初回 Ping に失敗しました", err, "params", SafeRepr(ctx, cfg))
        pool.Close()
        return nil, err
    }
    return pool, nil
}
```

ログには `SafeRepr` の戻り値 (host / port / user / dbname / sslmode のみ) を渡し、`BuildDSN` の戻り値は決してログに渡さない (F-10)。
context は cancel 伝播のために必ず引数で受け取る (F-18 が `internal/db/` 全公開関数に適用)。

## マイグレーション運用パターン (M0.3 追加)

開発フローは Makefile の 9 ターゲットで完結する。Q9 (起動と migrate 分離) のため、サーバ起動経路から `migrate up` を呼ばない。

```bash
# 初回 / バージョン更新時
make migrate-install                       # CLI を $(go env GOPATH)/bin に install
make migrate-create NAME=add_users_table   # 雛形生成 (Q4: 8 桁連番)
make migrate-up                            # 全 pending を適用
make migrate-up-one                        # 1 件だけ適用
make migrate-down                          # 直近 1 件をロールバック
make migrate-down-all                      # 全件ロールバック (危険、警告付き)
make migrate-version                       # 現在 version を表示
make migrate-status                        # graceful 表示 (no-version は exit 0、それ以外は通常 exit)
make migrate-force VERSION=<n>             # dirty 状態の強制リセット
```

起動シーケンスでは `dbmigrate.AssertClean` を呼び、dirty 検出で `os.Exit(1)`:

```go
// cmd/core/main.go (抜粋、F-13 start gate)
if err := dbmigrate.AssertClean(ctx, db.BuildDSN(ctx, &cfg.Database), migrationsSource, l); err != nil {
    if errors.Is(err, dbmigrate.ErrDirty) {
        l.Error(ctx, "schema_migrations is dirty: 'make migrate-force VERSION=<n>' で復旧してください", err)
    } else {
        l.Error(ctx, "schema_migrations の整合性確認に失敗しました", err)
    }
    return exitError
}
```

dirty 復旧手順:

1. ログから dirty 状態の version を特定
2. 当該 version の `up.sql` を見て中途半端に適用された DDL を手動巻き戻し
3. `make migrate-force VERSION=<n>` で `schema_migrations.dirty=false` に戻す
4. `make migrate-up` で再適用

## 統合テストパターン (M0.3 追加)

`core/internal/testutil/dbtest` に `NewPool` / `BeginTx` / `RollbackTx` を提供。各テストは TX 単位で隔離 (T-81)、defer Rollback で残留 state なし (T-82)。

```go
//go:build integration
// File: internal/db/db_integration_test.go

package db_test

import (
    "testing"
    "github.com/mktkhr/id-core/core/internal/testutil/dbtest"
)

func TestSomething_Integration(t *testing.T) {
    ctx, pool := dbtest.NewPool(t)            // 接続失敗時、CI=fatal、ローカル=skip
    tx := dbtest.BeginTx(t, ctx, pool)
    defer dbtest.RollbackTx(t, ctx, tx)

    if _, err := tx.Exec(ctx, "INSERT INTO ...", ...); err != nil {
        t.Fatalf("INSERT: %v", err)
    }
    // 検証...
}
```

実行: `make -C core test-integration` (内部で `go test -p 1 -race -tags integration ./...`)。
`-p 1` (package 単位順次実行) は将来的にテーブル truncate 等のグローバル状態を共有するパッケージが導入された場合の安全性確保。

CI と local の挙動分離:

- CI: `TEST_DB_REQUIRED=1` を設定し、DB 接続失敗を skip ではなく fail に
- ローカル: `TEST_DB_REQUIRED` 未設定で、開発者が DB を立てない時のユニットテスト実行を妨げない

## context.Context での DB 関連 ID 伝播 (M0.3 追加)

`internal/db/` / `internal/dbmigrate/` の全公開関数は `ctx context.Context` を第 1 引数に受け取る (F-18)。`internal/testutil/dbtest/` は **F-18 の適用例外**: テスト用ヘルパーは `*testing.T` を起点とし、`NewPool(t)` が `(ctx, *pgxpool.Pool)` を return する形 (Go の `httptest` 等の慣習に整合)。詳細は `core/internal/testutil/dbtest/helper.go` の `DatabaseURL` ドキュメントコメント参照。

ctx 伝播により:

- HTTP middleware が付与した `request_id` が DB 経路まで伝播し、相関ログが取得可能
- 起動シーケンスで付与した `event_id` が migrate / Ping ログまで伝播
- cancel / timeout が SQL 実行レベルまで伝播 (`pool.Ping(ctx)` は ctx を honor、SELECT/INSERT も同様)

Domain 層 (将来導入) は ctx から ID を取り出すのみ、ロガーへの直呼び出しは禁止 (F-14、M0.2 で確立)。
