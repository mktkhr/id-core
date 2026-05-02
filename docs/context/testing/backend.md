# バックエンドテスト規約

> 最終更新: 2026-05-02 (M0.2: id-core / Go のテスト規約最低限を反映)

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
