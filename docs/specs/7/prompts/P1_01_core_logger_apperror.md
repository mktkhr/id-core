# P1: core/internal/logger + core/internal/apperror パッケージ実装

- 対応 Issue: #12 (【実装】M0.2 ログ・エラー基盤: logger + apperror パッケージ)
- 元要求: #7
- 親設計書: `docs/specs/7/index.md`
- マイルストーン: M0.2 (Phase 0: スパイク)

## 絶対ルール

- **コードベース探索禁止**: Grep, Glob, Read による既存コードの探索的な読み取りを行わない
- **Explore エージェント起動禁止**: サブエージェントによるコードベース調査を行わない
- 実装に必要な情報は全て、以下の context ファイルとこのプロンプト内に記載済み
- context に記載がなく実装判断できない場合は、探索ではなく**ユーザーに質問**すること
- 変更対象ファイルの直接の Read / Edit のみ許可 (探索目的の Read は禁止)
- **Codex レビューをスキップしない**: 各作業ステップに Codex レビューが含まれている。レビューを受けずに次のステップに進むことは禁止
- **完了条件のタスク化**: 作業開始前に「完了条件」セクションの各項目を TaskCreate で登録し、各ステップ完了時に TaskUpdate で状態を更新すること。タスク化せずに作業を開始することは禁止

### プロジェクト固有の禁止事項 (恒久ルール)

- **UUID v4 禁止**: `request_id` / `event_id` / DB 主キーいずれも **UUID v7 のみ** を使う。Go では `github.com/google/uuid` v1.6 以上の `uuid.NewV7()` を使う。`uuid.New()` / `uuid.NewRandom()` (= v4) は呼ばない
- **`log.Fatal*` 禁止**: `core/` 配下で `log.Fatal` / `log.Fatalf` / `log.Fatalln` を使わない。本 Issue では使用箇所はないが、新規追加禁止
- **`time.Local = time.UTC` 禁止**: プロセス全体への副作用となるため使わない。代わりに `slog.NewJSONHandler` の `ReplaceAttr` フックで `time.Time` を `t.UTC().Format(time.RFC3339Nano)` に変換する
- **redact 部分一致禁止**: deny-list キーは **case-insensitive かつ完全一致** で照合する。`accessusername` のようなフィールドが `access_token` の部分一致で誤検出されてはならない
- **Domain 層ログ禁止**: 本 Issue では Domain 層の実装はないが、後続 Issue でロガーを呼ぶのは middleware / handler / infrastructure 層のみ。logger パッケージが提供する API は context 経由で `request_id` / `event_id` を取得する設計とする
- **依存追加の独断禁止**: `go.mod` に新規依存を追加する場合 (例: `github.com/google/uuid`) はユーザーに事前承認を取る。バージョンも明示

### コミット時の禁止事項

- コミットメッセージに `Co-Authored-By:` trailer を入れない (恒久ルール)
- main ブランチへ直接 push しない (feature ブランチ + PR + Codex レビュー必須、`.rulesync/rules/pr-review-policy.md` 準拠)

## 作業ステップ (この順序で実行すること)

### ステップ 0: 前提セットアップ

1. 作業ブランチ `feature/m0.2-impl-logger-apperror` を `main` から切る
2. `core/go.mod` に `github.com/google/uuid` v1.6+ を追加する**前にユーザーへ承認を求める** (バージョンも提示)。承認後に `go get github.com/google/uuid@v1.6.0` を実行
3. `make -C core build && make -C core test` がベースラインで pass することを確認

> **Codex レビュー対象外**: ステップ 0 はブランチ作成・依存追加承認・ベースライン確認のみで Go コード変更を伴わない。レビューはステップ 1 以降で実施する。

### ステップ 1: `core/internal/apperror/` 実装

`apperror` は `logger` の redact 連携で参照されるため先に実装する。

1. テストを先に書く: `apperror_test.go` で T-47〜T-52 を実装 (失敗確認)
2. 実装: `apperror.go` (CodedError 型 / `New` / `WithDetails` / `Unwrap`) と `response.go` (F-7 基本形 JSON シリアライザ)
3. `make -C core lint test` が pass することを確認
4. **Codex レビューを実行** (コマンドは末尾)
5. 指摘を対応してから次のステップへ

### ステップ 2: `core/internal/logger/` 実装 (基盤)

1. テストを先に書く: `logger_test.go` で T-1〜T-5 (初期化・フォーマット・time フィールド・副作用) を実装 (失敗確認)
2. 実装: `logger.go` (薄い独自インターフェース) + `format.go` (`CORE_LOG_FORMAT=json|text` 切替、`ReplaceAttr` で UTC 強制)
3. `make -C core lint test` が pass
4. **Codex レビューを実行**
5. 指摘を対応してから次のステップへ

### ステップ 3: `core/internal/logger/context.go` (context 伝播)

1. テストを先に書く: `context_test.go` で T-6〜T-9 を実装 (失敗確認)
2. 実装: `WithRequestID(ctx, id)` / `RequestIDFrom(ctx)` / `WithEventID(ctx, id)` / `EventIDFrom(ctx)`、logger の `Info` 等が context から自動で `request_id` / `event_id` を attr 化
3. `make -C core lint test` が pass
4. **Codex レビューを実行**
5. 指摘を対応してから次のステップへ

### ステップ 4: `core/internal/logger/redact.go` (deny-list redactor)

1. テストを先に書く: `redact_test.go` で T-10〜T-18 を実装 (失敗確認)
2. 実装: deny-list 完全リスト (本プロンプトの「設計仕様 / redact 規約」参照)、HTTP ヘッダ照合 (case-insensitive)、JSON body / map / 配列の再帰走査、完全一致のみ (部分一致禁止)、`[REDACTED]` 固定値置換
3. `make -C core lint test` が pass
4. **Codex レビューを実行**
5. 指摘を対応してから次のステップへ

### ステップ 5: `core/internal/logger/fallback.go` (出力失敗時フォールバック)

1. テストを先に書く: `fallback_test.go` で T-19〜T-21 を実装 (失敗確認)
2. 実装: stdout writer エラー時に stderr へフォールバック書き込み、`sync/atomic` で drop counter を保持、エクスポートする `DropCount() int64` を提供 (M1.x のメトリクス連携で利用)
3. `make -C core lint test` が pass
4. **Codex レビューを実行**
5. 指摘を対応してから次のステップへ

### ステップ 6: 全体テスト + ベースラインの確認 + 最終 Codex レビュー

1. `make -C core build && make -C core test && make -C core lint` 全件 pass
2. `grep -rn "log\.Fatal" core/` の出力が**増えていない**ことを確認 (本 Issue では cmd/core/main.go の既存利用は temporally 残してよい。完全排除は P2 で行う)
3. `grep -rn "uuid\.New[^V]" core/` の出力が 0 件 (UUID v4 が混入していない)
4. PR 作成 (`gh pr create` with `--assignee` + `--label`)
5. **`/pr-codex-review {PR 番号}` で Codex に PR 全体 (差分 + description) をレビューさせる**。これが本フェーズの最終 Codex レビュー (絶対ルール「Codex レビュー必須」を満たす全体検査)
6. ゲート通過 (CRITICAL=0 / HIGH=0 / MEDIUM<3) → ユーザー承認 → マージ

## 実装コンテキスト

```
CONTEXT_DIR="docs/context"
```

実装時に参照する context ファイル (これだけで足りるよう設計済み):

- `${CONTEXT_DIR}/app/architecture.md` (id-core 全体像、不読でも実装可)
- `${CONTEXT_DIR}/backend/conventions.md` (Go 1.22+ 規約、`internal/<feature>/` 配置、Makefile 規約)
- `${CONTEXT_DIR}/backend/patterns.md` (httptest パターン、`t.Setenv` 制約、外部テストパッケージ命名)

設計書: `docs/specs/7/index.md`

適用範囲: `core/internal/logger/` および `core/internal/apperror/` のみ (新規)

## 前提条件

- M0.1 (Issue #1, #2) 完了済み: `core/cmd/core/main.go` / `core/internal/{config,server,health}/` が存在し、`make -C core build && make test && make lint` がベースラインで pass する
- 本 Issue (#12) 完了後に Issue #13 (middleware + server 統合) に着手可能になる
- Issue #14 (契約テスト + context 更新) は #12 + #13 完了後

## 不明点ハンドリング

- 矛盾・欠落・未定義がある場合は作業を止める
- 勝手な推測で実装を進めない
- 質問時は: 止まっている作業単位 / 判断が必要な論点 / 選択肢を整理
- 特に以下が判明した時点で**即停止してユーザーに質問**する:
  - `github.com/google/uuid` のバージョン選定で迷う
  - `slog.NewJSONHandler` の `ReplaceAttr` で UTC 変換できないケース
  - redact のキー照合で部分一致が必要に見えるケース
  - 設計書 (`docs/specs/7/index.md`) と本プロンプトに矛盾を発見

## タスク境界

### 本プロンプトで実装する範囲

- `core/internal/logger/` の全ファイル (`logger.go` / `context.go` / `format.go` / `redact.go` / `fallback.go` + 各 `_test.go`)
- `core/internal/apperror/` の全ファイル (`apperror.go` / `response.go` / `apperror_test.go`)
- `core/go.mod` への `github.com/google/uuid` 追加 (ユーザー承認後)
- 上記ファイルの単体テスト T-1〜T-21 (logger 21 ケース) + T-47〜T-52 (apperror 6 ケース) = **計 27 ケース**

### 本プロンプトでは実装しない範囲

- HTTP middleware (`internal/middleware/`) → P2 (Issue #13)
- `server.go` の middleware チェーン組込 → P2
- `cmd/core/main.go` の `log.Fatal` 全廃 → P2
- F-16 ログスキーマ契約テスト → P3 (Issue #14)
- `docs/context/` 4 ファイルの更新 → P3
- `core/Makefile` の lint で grep 検査追加 → P3
- `core/README.md` の規約入口段落追加 → P3

## 設計仕様

### 関連要件 (F-N) の引用

| ID   | 内容                                                                                                                                                                                                                                   |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| F-1  | 構造化ログが標準出力に JSON Lines 形式で出力される (本番想定モード)。クライアント由来文字列はすべて JSON エンコーダ経由で出力し、改行・制御文字によるログインジェクションを防ぐ                                                        |
| F-2  | 開発時用に人間可読フォーマットへ切り替えるモードが用意されている (環境変数 `CORE_LOG_FORMAT=json\|text`、デフォルト `json`)                                                                                                            |
| F-3  | HTTP 経路で発生する全ログレコードに `request_id` を必須付与する。最低限のフィールド: `time` (RFC3339Nano UTC) / `level` (DEBUG/INFO/WARN/ERROR) / `msg` / `request_id` / `method` / `path` / `status` / `duration_ms` (アクセスログ系) |
| F-4  | HTTP 経路外のログレコードには HTTP 由来の `request_id` を入れず、代わりに `event_id` (起動毎・ジョブ毎に発番される一意 ID) を必須付与する                                                                                              |
| F-7  | 内部 API のエラーレスポンス JSON 形式は `{ "code": string, "message": string, "details"?: object, "request_id": string }`。`details` は object / array に限定                                                                          |
| F-13 | シークレットはログ出力時に redact される。完全リストは下記 Q8 採用値                                                                                                                                                                   |
| F-14 | Domain 層でロガーを直接呼ばない。logger は `context.Context` から `request_id` / `event_id` を取得して付与                                                                                                                             |

### 関連論点 (Q-N / D-N) の決定値

| ID  | 決定                                                                                                                                                                                                           |
| --- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Q1  | ロガー実装: **`log/slog` (Go 標準)**                                                                                                                                                                           |
| Q2  | フォーマット切替: **`CORE_LOG_FORMAT=json\|text`** (デフォルト `json`)                                                                                                                                         |
| Q3  | 一意 ID 生成: **UUID v7** (v4 禁止、`github.com/google/uuid` v1.6+ の `uuid.NewV7()`)                                                                                                                          |
| Q4  | `time` フォーマット: **RFC3339Nano (UTC、`Z` suffix 強制)**。`slog.NewJSONHandler` の `ReplaceAttr` で `t.UTC().Format(time.RFC3339Nano)` に変換 (`time.Local = time.UTC` の全プロセス副作用は禁止)            |
| Q5  | 内部 API エラー JSON: F-7 基本形を **`internal/apperror/`** パッケージで実装                                                                                                                                   |
| Q7  | ログレベル使い分け: DEBUG (本番無効、開発・調査) / INFO (業務イベント) / WARN (4xx) / ERROR (5xx・panic・予期しないエラー)                                                                                     |
| Q8  | redact 完全リスト (下記参照)                                                                                                                                                                                   |
| Q9  | ログ出力失敗時: **stderr フォールバック + atomic drop counter**。リクエスト処理は継続。drop counter は内部保持、外部公開は M1.x で                                                                             |
| D2  | logger 公開 API: **薄い独自インターフェース** (`logger.Info(ctx, msg, ...args)` / `logger.Error(ctx, msg, err, ...args)`)。内部実装は `slog.Logger`、context から `request_id` / `event_id` を自動付与する責務 |

### redact 規約 (Q8 完全一覧)

| 適用面                  | 対象キー                                                                                                                                                                                                                          | 照合規則                                             |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------- |
| HTTP リクエストヘッダ   | `Authorization`, `Cookie`, `Set-Cookie`, `Proxy-Authorization`, `X-Api-Key`, `X-Auth-Token`                                                                                                                                       | case-insensitive                                     |
| JSON body / `details`   | `password`, `current_password`, `new_password`, `access_token`, `refresh_token`, `id_token`, `code`, `code_verifier`, `client_secret`, `assertion`, `client_assertion`, `private_key`, `secret`, `api_key`, `jwt`, `bearer_token` | case-insensitive かつ完全一致 / ネスト・配列再帰走査 |
| query / form パラメータ | 同上 (body と同一リスト)                                                                                                                                                                                                          | case-insensitive かつ完全一致                        |
| error chain メッセージ  | 同上 (文字列 fuzzy 検出は行わず、構造化フィールド経由でのみ redact)                                                                                                                                                               | キー一致のみ                                         |

**redact 値**: 文字列 `[REDACTED]` 固定で置換。長さや存在情報を漏らさない (`****` のようなマスキング禁止)。

### `internal/apperror/` 実装の詳細

```go
// apperror.go (実装の方向性。完全な署名は実装者が決めてよい)

// CodedError は内部 API のエラーレスポンスを表す型。
type CodedError struct {
    code    string         // SCREAMING_SNAKE_CASE (例: "INVALID_PARAMETER")
    message string         // 人間可読 (本スコープでは日本語、i18n は M3.3 以降)
    details map[string]any // optional, object/array のみ。シークレット禁止 (redact 連携)
    cause   error          // error chain (errors.Is / errors.As 互換)
}

// New は CodedError を新規作成する。
func New(code, message string) *CodedError

// WithDetails は details を付与した新しいインスタンスを返す (immutable)。
func (e *CodedError) WithDetails(details map[string]any) *CodedError

// Wrap は原因 error をラップして error chain を作る。
func (e *CodedError) Wrap(cause error) *CodedError

// Code / Message / Details ゲッター
// Unwrap は errors.Is / errors.As 互換のために cause を返す。
```

```go
// response.go (実装の方向性)

// Response は F-7 基本形 JSON のシリアライズ用構造体。
// HTTP middleware (P2) から呼び出されてレスポンスボディに書き込まれる。
type Response struct {
    Code      string         `json:"code"`
    Message   string         `json:"message"`
    Details   map[string]any `json:"details,omitempty"`
    RequestID string         `json:"request_id"`
}

// ToResponse は CodedError を Response に変換する。
// request_id は context から取得する責務は呼び出し側 (middleware) にある。
func ToResponse(e *CodedError, requestID string) Response

// WriteJSON は w に Response を JSON Lines として書き込み、Content-Type を設定する。
// (P2 の middleware から呼ばれる)
func WriteJSON(w http.ResponseWriter, status int, e *CodedError, requestID string) error
```

固定の `code` 値 (本 Issue で定義する最低限):

- `INTERNAL_ERROR`: panic 時の固定 code (P2 の recover middleware で使用)
- 他のコードは後続 Issue / マイルストーンで追加

### `internal/logger/` 実装の詳細

```go
// logger.go (薄い独自インターフェース)

// Logger は本プロジェクトのログ出力 API。
// 内部実装は slog.Logger だが、context から request_id / event_id を自動付与する。
type Logger struct {
    // 内部 slog.Handler (JSON or Text)
}

func New(format Format, w io.Writer) *Logger // 主要コンストラクタ
func Default() *Logger                        // CORE_LOG_FORMAT を読んで初期化

func (l *Logger) Info(ctx context.Context, msg string, args ...any)
func (l *Logger) Warn(ctx context.Context, msg string, args ...any)
func (l *Logger) Error(ctx context.Context, msg string, err error, args ...any) // err 専用引数
func (l *Logger) Debug(ctx context.Context, msg string, args ...any)
```

```go
// context.go (context 伝播)

type ctxKey struct{ name string }

var (
    requestIDKey = ctxKey{"request_id"}
    eventIDKey   = ctxKey{"event_id"}
)

func WithRequestID(ctx context.Context, id string) context.Context
func RequestIDFrom(ctx context.Context) string // 取れなければ空文字

func WithEventID(ctx context.Context, id string) context.Context
func EventIDFrom(ctx context.Context) string // 取れなければ空文字
```

```go
// format.go (CORE_LOG_FORMAT 切替 + UTC ReplaceAttr)

type Format int

const (
    FormatJSON Format = iota
    FormatText
)

const envLogFormat = "CORE_LOG_FORMAT"

func FormatFromEnv() (Format, error) // CORE_LOG_FORMAT を読む。invalid はエラー
func newHandler(format Format, w io.Writer) slog.Handler // ReplaceAttr で time を UTC 化
```

```go
// redact.go (deny-list redactor)

const RedactedValue = "[REDACTED]"

// HeaderKeysToRedact は HTTP リクエストヘッダの redact 対象 (case-insensitive)。
var HeaderKeysToRedact = []string{
    "Authorization",
    "Cookie",
    "Set-Cookie",
    "Proxy-Authorization",
    "X-Api-Key",
    "X-Auth-Token",
}

// FieldKeysToRedact は body / query / form / details の redact 対象 (case-insensitive 完全一致)。
var FieldKeysToRedact = []string{
    "password",
    "current_password",
    "new_password",
    "access_token",
    "refresh_token",
    "id_token",
    "code",
    "code_verifier",
    "client_secret",
    "assertion",
    "client_assertion",
    "private_key",
    "secret",
    "api_key",
    "jwt",
    "bearer_token",
}

// RedactHeaders は http.Header のディープコピーを作り、対象キーを [REDACTED] に置換する。
func RedactHeaders(h http.Header) http.Header

// RedactMap は map[string]any を再帰走査して対象キーを [REDACTED] に置換する (object/array)。
// 元の map は変更しない (new map を返す)。
func RedactMap(m map[string]any) map[string]any
```

```go
// fallback.go (ログ出力失敗時)

// FallbackWriter は writer を wrap し、書き込み失敗時に stderr へフォールバックする。
// stderr も失敗した場合は drop count を atomic に増分する。
type FallbackWriter struct {
    primary io.Writer
    drops   atomic.Int64
}

func NewFallbackWriter(primary io.Writer) *FallbackWriter
func (w *FallbackWriter) Write(p []byte) (int, error) // ログ自身の書き込みは失敗を返さない (継続)
func (w *FallbackWriter) DropCount() int64
```

## テスト観点

### `internal/logger/` のテスト (T-1〜T-21)

| #    | カテゴリ     | 観点                                                            | 期待                                                                               | 関連          |
| ---- | ------------ | --------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ------------- |
| T-1  | 正常系       | `CORE_LOG_FORMAT=json` (デフォルト) で初期化                    | `slog.NewJSONHandler` ベース、出力は 1 行 JSON で `time` / `level` / `msg` を含む  | F-1, Q1, Q2   |
| T-2  | 正常系       | `CORE_LOG_FORMAT=text` で初期化                                 | `slog.NewTextHandler` ベース、出力は `key=value` 形式                              | F-2, Q2       |
| T-3  | 異常系       | `CORE_LOG_FORMAT=invalid` で初期化                              | エラーが返る (`fmt.Errorf` で原因含む)                                             | F-2, Q2       |
| T-4  | 正常系       | `time` フィールドのフォーマット                                 | `2026-05-02T01:00:00.123456789Z` 形式 (RFC3339Nano、UTC、`Z` suffix)               | F-3, Q4       |
| T-5  | セキュリティ | プロセス全体への副作用がない                                    | ロガー初期化前後で `time.Local` が変化しない                                       | Q4            |
| T-6  | 正常系       | `WithRequestID(ctx, "v7-uuid")` 後に `RequestIDFrom(ctx)`       | 元の値が取り出せる                                                                 | F-5, F-14     |
| T-7  | 正常系       | `WithEventID(ctx, "v7-uuid")` 後に `EventIDFrom(ctx)`           | 元の値が取り出せる                                                                 | F-4, F-14     |
| T-8  | 準正常系     | request_id を入れていない context から `RequestIDFrom(ctx)`     | 空文字 (panic しない)                                                              | F-14          |
| T-9  | 正常系       | logger 経由でログ出力時に context から自動付与                  | `request_id` (HTTP 経路) または `event_id` (非 HTTP 経路) がログレコードに含まれる | F-3, F-4, D2  |
| T-10 | セキュリティ | HTTP ヘッダ `Authorization: Bearer xxx` を redact               | `[REDACTED]` 置換                                                                  | F-13, Q8      |
| T-11 | セキュリティ | HTTP ヘッダ `authorization: bearer xxx` (case 違い) を redact   | case-insensitive で `[REDACTED]` 置換                                              | F-13, Q8      |
| T-12 | セキュリティ | JSON body `{"password": "secret"}` を redact                    | `password` 値が `[REDACTED]` 置換                                                  | F-13, Q8      |
| T-13 | セキュリティ | ネストした JSON `{"outer": {"client_secret": "xxx"}}` を redact | 再帰走査で `client_secret` 値が `[REDACTED]`                                       | F-13, Q8      |
| T-14 | セキュリティ | 配列内の JSON `{"creds": [{"access_token": "x"}]}` を redact    | 配列要素も走査して `access_token` 値が `[REDACTED]`                                | F-13, Q8      |
| T-15 | セキュリティ | redact 対象キーの全 16 フィールドを網羅 (テーブル駆動)          | Q8 完全リスト全件が `[REDACTED]`                                                   | F-13, Q8      |
| T-16 | セキュリティ | redact 対象キーの全 6 ヘッダを網羅                              | Q8 完全リスト 6 件が case-insensitive で `[REDACTED]`                              | F-13, Q8      |
| T-17 | 準正常系     | redact 対象でないキー `username` は変換しない                   | 値はそのまま (部分一致禁止: `accessusername` は `access_token` に誤検知しない)     | F-13, Q8      |
| T-18 | セキュリティ | redact 対象キーが存在しない場合の出力                           | 元の構造のまま `[REDACTED]` 置換は発生しない                                       | F-13          |
| T-19 | 異常系       | stdout writer がエラーを返す状況をモックして再現                | リクエスト処理は継続 (panic しない)、stderr に最低限のフォールバック行が出力される | NFR可用性, Q9 |
| T-20 | 異常系       | stdout / stderr 両方失敗する状況                                | 処理継続、drop counter が増加 (`atomic.LoadInt64` で確認)                          | NFR可用性, Q9 |
| T-21 | 正常系       | 通常出力時の drop counter                                       | 0 のまま増加しない                                                                 | Q9            |

### `internal/apperror/` のテスト (T-47〜T-52)

| #    | カテゴリ     | 観点                                                                            | 期待                                                                                             | 関連     |
| ---- | ------------ | ------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | -------- |
| T-47 | 正常系       | `apperror.New(code, message)` を JSON シリアライズ                              | `{"code":"...","message":"...","request_id":"..."}` (details なし)                               | F-7, Q5  |
| T-48 | 正常系       | `apperror.New(...).WithDetails(map[string]any{"field":"value"})` をシリアライズ | `details` フィールドに object 形式で含まれる                                                     | F-7      |
| T-49 | セキュリティ | `details` にシークレット (例: `password`) を入れた場合                          | logger redact 経由で `[REDACTED]` 置換 (連携)                                                    | F-13     |
| T-50 | 正常系       | `details` 型制約 (object / array のみ)                                          | string / number / bool を直接 `details` にすると compile error または runtime バリデーション失敗 | F-7      |
| T-51 | 正常系       | error chain の `errors.Is` / `errors.As` 互換性                                 | `Unwrap()` メソッドで原因 error が取得できる                                                     | F-7      |
| T-52 | 異常系       | request_id が context にない場合のエラーレスポンス生成                          | `request_id` フィールドは空文字、ボディは正しい JSON                                             | F-7, F-9 |

## Codex レビューコマンド (各ステップで使用)

各ステップ完了時に、git diff の差分のみをレビューさせる。フェーズ全体を一度にレビューしない。

```bash
# CONTEXT_DIR を変数化 (リポジトリルートからの相対パス)
CONTEXT_DIR="docs/context"

codex exec --sandbox read-only "Review the staged + unstaged Go diff in core/internal/{logger,apperror}.

Context to read first:
- ${CONTEXT_DIR}/app/architecture.md (id-core 全体像)
- ${CONTEXT_DIR}/backend/conventions.md (Go 規約)
- ${CONTEXT_DIR}/backend/patterns.md (httptest パターン、t.Setenv 制約)
- docs/specs/7/index.md (本フェーズ設計書)
- docs/specs/7/prompts/P1_01_core_logger_apperror.md (本プロンプト)

Then review the current diff (use git diff). Check:
1) TDD compliance: テストが先行コミットされているか / 各 T-N がカバーされているか
2) Project policy compliance:
   - UUID v4 不使用 (uuid.New / uuid.NewRandom 禁止、uuid.NewV7 のみ)
   - log.Fatal* 不使用 (新規追加禁止)
   - time.Local = time.UTC 不使用 (ReplaceAttr で UTC 変換すること)
   - redact 部分一致禁止 (case-insensitive 完全一致のみ、ネスト走査必須)
3) Go の慣用句: error wrapping (%w)、context 伝播、外部テストパッケージ命名
4) シークレット安全性: ログ・エラーレスポンスにスタックトレースや内部パス漏洩がないか
5) インターフェース設計: D2 の薄い独自 IF が slog.Logger を適切に隠蔽しているか
6) 依存追加: go.mod の差分はユーザー承認済みのもののみか

Report issues as CRITICAL/HIGH/MEDIUM/LOW in Japanese with file:line references."
```

## 完了条件

- [ ] 作業ブランチ `feature/m0.2-impl-logger-apperror` を作成
- [ ] `github.com/google/uuid` v1.6+ の追加についてユーザー承認を取得
- [ ] `core/go.mod` に `github.com/google/uuid` 追加 + `go.sum` 更新
- [ ] `core/internal/apperror/apperror.go` 実装
- [ ] `core/internal/apperror/response.go` 実装
- [ ] `core/internal/apperror/apperror_test.go` で T-47〜T-52 を実装、全件 pass
- [ ] `core/internal/logger/logger.go` 実装
- [ ] `core/internal/logger/format.go` 実装
- [ ] `core/internal/logger/context.go` 実装
- [ ] `core/internal/logger/redact.go` 実装
- [ ] `core/internal/logger/fallback.go` 実装
- [ ] `core/internal/logger/{logger,context,redact,fallback}_test.go` で T-1〜T-21 を実装、全件 pass
- [ ] `make -C core build && make -C core test && make -C core lint` 全件 pass
- [ ] `grep -rn "uuid\.New[^V]" core/` の出力が 0 件 (UUID v4 が混入していない)
- [ ] 各ステップで Codex レビューを実施、CRITICAL=0 / HIGH=0 / MEDIUM<3 を確認
- [ ] PR 作成 (`gh pr create` with `--assignee` + `--label "種別:実装" --label "対象:基盤"`)
- [ ] `/pr-codex-review {番号}` でゲート通過
- [ ] PR の Test plan を実機確認して `[x]` に書き換え
- [ ] PR をマージ (Issue #12 が自動 close)
- [ ] 親 Issue #7 の task list で #12 にチェックが付くことを確認
