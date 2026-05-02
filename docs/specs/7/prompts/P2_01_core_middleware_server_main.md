# P2: core/internal/middleware + server 統合 + log.Fatal 全廃

- 対応 Issue: #13 (【実装】M0.2 middleware + server 統合 + log.Fatal 全廃)
- 元要求: #7
- 親設計書: `docs/specs/7/index.md`
- 先行プロンプト: `docs/specs/7/prompts/P1_01_core_logger_apperror.md`
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

- **UUID v4 禁止**: `uuid.NewV7()` のみ使用
- **`log.Fatal*` 禁止 + 全廃**: 本 Issue の主目的。`core/cmd/core/main.go` の既存 `log.Fatalf` 2 箇所を構造化ロガー (`logger.Error` + `os.Exit(1)`) に置換する。完了条件で `grep -rn "log\.Fatal" core/` 0 件
- **`time.Local = time.UTC` 禁止**
- **redact 部分一致禁止**
- **Domain 層ログ禁止**: middleware と handler のみがロガーを呼ぶ

### コミット時の禁止事項

- コミットメッセージに `Co-Authored-By:` trailer を入れない
- main ブランチへ直接 push しない (feature ブランチ + PR + Codex レビュー必須)

## 作業ステップ (この順序で実行すること)

### ステップ 0: 前提セットアップ

1. Issue #12 (P1) のマージ済みを確認 (`core/internal/{logger,apperror}/` が存在し pass する)
2. 作業ブランチ `feature/m0.2-impl-middleware-server` を `main` から切る
3. `make -C core build && make -C core test` がベースラインで pass

> **Codex レビュー対象外**: ステップ 0 はブランチ作成・前提確認のみで Go コード変更を伴わない。レビューはステップ 1 以降で実施する。

### ステップ 1: `internal/middleware/request_id.go` 実装 (F-5/F-6)

1. テストを先に書く: `request_id_test.go` で T-22〜T-32 を実装 (失敗確認)
2. 実装: クライアント `X-Request-Id` の妥当性検証 (本プロンプトの「設計仕様」参照)、不正値は破棄して **UUID v7** で再生成、`context.Context` に格納、レスポンスヘッダ `X-Request-Id` を **next 呼び出し前**に先行設定 (handler が WriteHeader を呼ぶと以降のヘッダ変更が無視されるため)
3. 不正だった元値はサニタイズ (制御文字を Unicode エスケープ、長さ 128 オクテットで切詰め) のうえ、ログにのみ別フィールド `client_request_id` として残す
4. `make -C core lint test` が pass
5. **Codex レビューを実行**
6. 指摘を対応してから次のステップへ

### ステップ 2: `internal/middleware/access_log.go` 実装 (F-3/F-11/D3)

1. テストを先に書く: `access_log_test.go` で T-40〜T-46 を実装 (失敗確認)
2. 実装: `defer` でリクエスト終了時に 1 行 INFO/WARN/ERROR ログを出力 (D3: 終了時のみ 1 行)。フィールド: `time` (RFC3339Nano UTC) / `level` / `msg=access` / `request_id` / `method` / `path` / `status` / `duration_ms`。レベル自動判定: 5xx=ERROR / 4xx=WARN / それ以外=INFO (F-11 / Q7)
3. status 取得のため `http.ResponseWriter` をラップする (例: 内部 `responseWriter` 構造体で `WriteHeader` の status を捕捉)
4. クエリ文字列に redact 対象キーが含まれる場合 (例: `?code=secret`)、ログ出力時に `[REDACTED]` 化する (T-45)
5. `make -C core lint test` が pass
6. **Codex レビューを実行**
7. 指摘を対応してから次のステップへ

### ステップ 3: `internal/middleware/recover.go` 実装 (F-9/F-10)

1. テストを先に書く: `recover_test.go` で T-33〜T-39 を実装 (失敗確認)
2. 実装: `defer recover()` で panic 捕捉 → 内部に level=ERROR で構造化ログを出す (`request_id` / `error` 値 / `stack_trace` フィールドに `runtime.Stack(...)` の結果)。スタックトレースは内部ログのみ、HTTP レスポンスには絶対に載せない (F-10)
3. クライアントには HTTP 500 + `Content-Type: application/json; charset=utf-8` + F-7 基本形 JSON で固定メッセージ + `request_id` を返す: `apperror.WriteJSON(w, 500, apperror.New("INTERNAL_ERROR", "内部エラーが発生しました"), requestID)`
4. panic value が `error` 型なら `error` フィールドに wrap、それ以外は `fmt.Sprintf("%v", v)` で文字列化
5. `make -C core lint test` が pass
6. **Codex レビューを実行**
7. 指摘を対応してから次のステップへ

### ステップ 4: `internal/server/server.go` の middleware チェーン組込

1. テストを先に書く: `server_test.go` を更新し、middleware チェーンが組み込まれていることを統合的に検証 (T-53 が含まれる想定)
2. 実装: D1 順序 `request_id` → `access_log` → `recover` → `handler` で wrap する。コード例:

   ```go
   handler := http.NewServeMux()
   handler.HandleFunc("GET /health", health.Handler)

   // 内側から外側へ wrap (実行は外側から)
   var wrapped http.Handler = handler
   wrapped = middleware.Recover(logger, wrapped)
   wrapped = middleware.AccessLog(logger, wrapped)
   wrapped = middleware.RequestID(wrapped)

   srv := &http.Server{
       Addr:              fmt.Sprintf(":%d", cfg.Port),
       Handler:           wrapped,
       ReadHeaderTimeout: readHeaderTimeout,
   }
   ```

3. `internal/health/health.go` の `_ = json.NewEncoder(...)` の暫定コメントを解消 (write エラー時は logger に記録するか、`apperror` で処理。ただし `/health` のように handler がエラーを起こしにくい場所では受動的にログだけ残す方針で十分)
4. `make -C core lint test` が pass
5. **Codex レビューを実行**
6. 指摘を対応してから次のステップへ

### ステップ 5: `cmd/core/main.go` の `log.Fatal` 全廃

1. テストを先に書く: `main_test.go` を新規作成し T-63〜T-65 を実装 (失敗確認)。直接 `main()` をテストするのは難しいため、`config.Load` がエラー返却時に呼ばれる「終了処理関数」を分離するなどの設計で test 可能化する
2. 実装: 標準 `log` パッケージの利用を完全に排除。代わりに `logger.Default()` で構造化ロガーを初期化し、起動時に `event_id` (UUID v7) + `port` 付きの INFO ログを出す (msg は日本語、例: `"core サーバーを起動します"`)
3. エラー時は `logger.Error(ctx, "設定の読み込みに失敗しました", err)` + `os.Exit(1)` で異常終了 (`log.Fatalf` を使わない)
4. `grep -rn "log\.Fatal" core/` の出力が **0 件** であることを確認
5. `grep -rn "\"log\"" core/` の出力が main.go から消えていることを確認 (`internal/logger` のみが log/slog を使う)
6. `make -C core lint test` が pass
7. **Codex レビューを実行**
8. 指摘を対応してから次のステップへ

### ステップ 6: 統合テスト (T-53〜T-57)

1. テストを実装: `server_test.go` (または新規 `integration_test.go`) で middleware チェーン全体を組み立てた状態で `/health` を叩き、レスポンスヘッダ `X-Request-Id` 付与 / アクセスログ 1 行出力 / panic 経路で 500 + 固定メッセージ / 不正 `X-Request-Id` のサニタイズを検証
2. `make -C core lint test` 全件 pass
3. **Codex レビューを実行**
4. 指摘を対応してから次のステップへ

### ステップ 7: 最終確認 + 最終 Codex レビュー

1. `make -C core build && make -C core test && make -C core lint` 全件 pass
2. `grep -rn "log\.Fatal" core/` の出力が 0 件
3. M0.1 の `/health` 外形互換 (HTTP 200 + `{"status":"ok"}` + Content-Type) が維持されていることを `httptest` で確認
4. PR 作成 (`gh pr create` with `--assignee` + `--label`)
5. **`/pr-codex-review {PR 番号}` で Codex に PR 全体 (差分 + description) をレビューさせる**。これが本フェーズの最終 Codex レビュー (絶対ルール「Codex レビュー必須」を満たす全体検査)
6. ゲート通過 (CRITICAL=0 / HIGH=0 / MEDIUM<3) → ユーザー承認 → マージ

## 実装コンテキスト

```
CONTEXT_DIR="docs/context"
```

実装時に参照する context ファイル:

- `${CONTEXT_DIR}/app/architecture.md`
- `${CONTEXT_DIR}/backend/conventions.md` (Makefile 規約、環境変数、ロギング欄)
- `${CONTEXT_DIR}/backend/patterns.md` (httptest パターン、ServeMux パターン、`t.Setenv` 制約)

設計書: `docs/specs/7/index.md`

先行実装: `core/internal/logger/` および `core/internal/apperror/` (P1 = #12 で完成済み)

適用範囲:

- `core/internal/middleware/` (新規)
- `core/internal/server/server.go` (既存修正)
- `core/cmd/core/main.go` (既存修正、`log.Fatal` 全廃)
- `core/internal/health/health.go` (既存修正、暫定コメント解消)

## 前提条件

- **Issue #12 (P1) がマージ済み**: `core/internal/{logger,apperror}/` が完成し、`make test` が pass している状態
- M0.1 (Issue #1, #2) 完了済み
- 本 Issue (#13) 完了後に Issue #14 (P3: 契約テスト + context 更新 + Makefile + README) に着手可能になる

## 不明点ハンドリング

- 矛盾・欠落・未定義がある場合は作業を止める
- 推測で実装を進めない
- 質問時は: 止まっている作業単位 / 判断が必要な論点 / 選択肢を整理
- 特に以下が判明した時点で**即停止してユーザーに質問**する:
  - middleware の `http.ResponseWriter` ラップ方式で迷う (status 取得や hijacker 対応の必要性)
  - panic 時のスタックトレース取得方式 (`runtime.Stack` のサイズ制限等)
  - `cmd/core/main.go` の `os.Exit(1)` を直接呼ぶか、テスト可能な分離設計を取るか
  - `/health` の暫定コメント解消の処理方針 (受動的ログ記録 vs apperror 処理)

## タスク境界

### 本プロンプトで実装する範囲

- `core/internal/middleware/request_id.go` (+ `_test.go`)
- `core/internal/middleware/access_log.go` (+ `_test.go`)
- `core/internal/middleware/recover.go` (+ `_test.go`)
- `core/internal/server/server.go` (修正: middleware チェーン組込、D1 順序)
- `core/internal/server/server_test.go` (修正: 統合テスト追加 T-53〜T-57)
- `core/cmd/core/main.go` (修正: `log.Fatal` 全廃、構造化ロガー初期化、起動 INFO ログ + `event_id`)
- `core/cmd/core/main_test.go` (新規 / 修正: T-63〜T-65)
- `core/internal/health/health.go` (修正: 暫定コメント解消)
- 上記のテストケース T-22〜T-46 (middleware) + T-53〜T-57 (統合) + T-63〜T-65 (main) = **計 33 ケース**

### 本プロンプトでは実装しない範囲

- F-16 ログスキーマ契約テスト → P3 (Issue #14)
- `docs/context/` 4 ファイルの更新 → P3
- `core/Makefile` の lint で `grep -rn "log\.Fatal"` 検査追加 → P3
- `core/README.md` の規約入口段落追加 → P3

## 設計仕様

### 関連要件 (F-N) の引用

| ID   | 内容                                                                                                                                                                                                                                                                            |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| F-3  | HTTP 経路で発生する全ログレコードに `request_id` を必須付与。フィールド: `time` / `level` / `msg` / `request_id` / `method` / `path` / `status` / `duration_ms` (アクセスログ系)                                                                                                |
| F-5  | HTTP middleware で `request_id` を生成 (UUID v7) し `context.Context` に格納、レスポンスヘッダ `X-Request-Id` を必ず返す (4xx / 5xx / panic 時を含む)                                                                                                                           |
| F-6  | クライアント提供 `X-Request-Id` の妥当性基準: (a) 長さ ≤ 128 オクテット (b) 文字種 ASCII 印字可能 (`0x21`–`0x7E`、制御文字・改行・空白・タブ含まない) (c) 上記を満たす場合のみ採用、満たさない場合は破棄して新規生成。元値はサニタイズしてログに `client_request_id` として残す |
| F-9  | エラーハンドラ middleware が `panic` / 既知ドメインエラー / 未捕捉 error の 3 系統を全て規約 JSON で返す                                                                                                                                                                        |
| F-10 | panic 発生時はクライアントへ固定メッセージ + `request_id` のみ返す。スタックトレース・内部ファイルパス・実装詳細を HTTP レスポンスへ絶対に含めない。スタックトレースは内部の構造化ログ (level=ERROR) にのみ記録                                                                 |
| F-11 | 各種失敗ケース (panic / 4xx / 5xx) で、規約に沿った構造化ログが残る。最低ルール: 5xx および panic は `level=ERROR`、4xx は `level=WARN` 以下                                                                                                                                    |
| F-12 | M0.1 由来の `log.Fatal*` を `core/` 配下から完全に排除する。完了条件は `grep -rn "log\.Fatal" core/` 出力が 0 件                                                                                                                                                                |
| F-14 | Domain 層でロガーを直接呼ばない。ログ出力は middleware / handler / infrastructure 層が担当し、`context.Context` から `request_id` (または `event_id`) を取得して付与                                                                                                            |

### 関連論点 (Q-N / D-N) の決定値

| ID  | 決定                                                                                                                                                                |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Q3  | UUID v7 (v4 禁止)                                                                                                                                                   |
| Q6  | OIDC エラーレスポンスでの `request_id` 露出方法: ヘッダ `X-Request-Id` のみ (RFC 6749 / 6750 完全準拠)。本 Issue では OIDC 実装はないが、ヘッダ常時付与の原則を守る |
| Q7  | ログレベル: DEBUG (開発・本番無効) / INFO (業務イベント) / WARN (4xx) / ERROR (5xx・panic)                                                                          |
| D1  | middleware 構成順序: **`request_id` (最外側) → `access_log` → `recover` → `handler`**                                                                               |
| D3  | アクセスログ出力タイミング: 終了時のみ 1 行                                                                                                                         |

### D1 順序の根拠 (重要)

- `request_id` を最外側に置くことで panic 時を含む全ログレコードに `request_id` が付く
- `recover` を `access_log` の**内側**に置くことで、handler の panic は `recover` が捕捉して 500 応答に変換されてから `access_log` の `defer` に戻る。これにより `access_log` は最終 status (500) と level (ERROR) を正しく観測できる
- 逆順 (`access_log` → `recover` → `handler`) では `access_log.defer` が panic unwind 中に走るため `status=0` / level 誤判定になる

### `X-Request-Id` レスポンスヘッダの**先行設定**ルール

`request_id` middleware は `next.ServeHTTP` を呼ぶ**前**に `w.Header().Set("X-Request-Id", id)` を実行する。理由: handler が `WriteHeader` を呼んだ以降の `w.Header().Set(...)` は HTTP レスポンスに反映されない (Go の `net/http` 仕様)。

```go
func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := getOrGenerate(r.Header.Get("X-Request-Id"))
        ctx := logger.WithRequestID(r.Context(), id)
        w.Header().Set("X-Request-Id", id) // ★ next 呼び出し前
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 各 middleware の関数シグネチャ案

```go
// internal/middleware/request_id.go
func RequestID(next http.Handler) http.Handler

// internal/middleware/access_log.go
func AccessLog(l *logger.Logger, next http.Handler) http.Handler

// internal/middleware/recover.go
func Recover(l *logger.Logger, next http.Handler) http.Handler
```

### `cmd/core/main.go` の置換イメージ

```go
package main

import (
    "context"
    "log/slog"
    "os"

    "github.com/google/uuid"
    "github.com/mktkhr/id-core/core/internal/config"
    "github.com/mktkhr/id-core/core/internal/logger"
    "github.com/mktkhr/id-core/core/internal/server"
)

func main() {
    if err := run(); err != nil {
        // run() 内で既にログは出ている。ここは終了処理のみ
        os.Exit(1)
    }
}

func run() error {
    l, err := logger.Default()
    if err != nil {
        // 起動時のロガー初期化失敗は標準エラーに最終フォールバック
        slog.Default().Error("ロガー初期化に失敗しました", "error", err)
        return err
    }

    eventID, _ := uuid.NewV7()
    ctx := logger.WithEventID(context.Background(), eventID.String())

    cfg, err := config.Load()
    if err != nil {
        l.Error(ctx, "設定の読み込みに失敗しました", err)
        return err
    }

    srv := server.New(cfg, l) // server.New が logger を受け取るシグネチャに変更

    l.Info(ctx, "core サーバーを起動します", slog.String("addr", srv.Addr))
    if err := srv.ListenAndServe(); err != nil {
        l.Error(ctx, "サーバーの実行に失敗しました", err)
        return err
    }
    return nil
}
```

## テスト観点

### `internal/middleware/request_id.go` のテスト (T-22〜T-32)

| #    | カテゴリ     | 観点                                                              | 期待                                                                        | 関連     |
| ---- | ------------ | ----------------------------------------------------------------- | --------------------------------------------------------------------------- | -------- |
| T-22 | 正常系       | `X-Request-Id` ヘッダなし                                         | 新規 UUID v7 生成、context 注入、レスポンスヘッダに同値設定                 | F-5, Q3  |
| T-23 | 正常系       | クライアント `X-Request-Id: abc123-XYZ` (妥当)                    | 受け取った値をそのまま採用、context / レスポンスヘッダに同値                | F-5, F-6 |
| T-24 | 境界値       | クライアント `X-Request-Id` が 128 オクテット ちょうど            | 採用される                                                                  | F-6      |
| T-25 | 境界値       | クライアント `X-Request-Id` が 129 オクテット                     | 破棄、新規生成、ログに `client_request_id` (サニタイズ済) として残る        | F-6      |
| T-26 | 異常系       | クライアント `X-Request-Id` に改行 (`\n` / `\r`) を含む           | 破棄、新規生成、`client_request_id` に制御文字を Unicode エスケープして記録 | F-6, F-1 |
| T-27 | 異常系       | クライアント `X-Request-Id` に空白 (`0x20`) を含む                | 破棄、新規生成                                                              | F-6      |
| T-28 | 異常系       | クライアント `X-Request-Id` にタブ (`0x09`) を含む                | 破棄、新規生成                                                              | F-6      |
| T-29 | 異常系       | クライアント `X-Request-Id` に DEL 文字 (`0x7F`) を含む           | 破棄、新規生成 (`0x21`–`0x7E` 範囲外)                                       | F-6      |
| T-30 | 正常系       | レスポンスヘッダ `X-Request-Id` の常時付与 (200 / 4xx / 5xx 全て) | どの status でもヘッダが必ず付く                                            | F-5      |
| T-31 | 正常系       | UUID v7 生成方式の検証                                            | UUID 形式 (8-4-4-4-12)、バージョンビットが `7`                              | Q3       |
| T-32 | セキュリティ | サニタイズ後 `client_request_id` 値の長さ                         | 128 オクテット以下に切り詰められる (DoS 防止)                               | F-6      |

### `internal/middleware/recover.go` のテスト (T-33〜T-39)

| #    | カテゴリ     | 観点                                            | 期待                                                                                                                             | 関連           |
| ---- | ------------ | ----------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | -------------- |
| T-33 | 異常系       | handler が `panic("test")` した場合の HTTP 応答 | HTTP 500、Content-Type `application/json; charset=utf-8`、ボディは F-7 基本形 JSON (固定 `code` / 固定 `message` / `request_id`) | F-9, F-10, F-7 |
| T-34 | セキュリティ | panic レスポンスにスタックトレースが含まれない  | body 内に `goroutine` / `runtime/panic` / 内部ファイルパスの文字列が現れない                                                     | F-10           |
| T-35 | 異常系       | panic レスポンスに `request_id` が含まれる      | レスポンス JSON の `request_id` フィールドが context の値と一致                                                                  | F-9, F-5       |
| T-36 | 異常系       | panic 時の内部ログ                              | level=ERROR、`msg` (日本語)、`request_id`、`error` 値、`stack_trace` フィールドにスタック情報を含む                              | F-10, F-11     |
| T-37 | 正常系       | handler が panic しない場合                     | recover の defer は何もしない (パススルー)                                                                                       | F-9            |
| T-38 | 異常系       | panic value が `error` 型の場合                 | error chain を `error` フィールドに記録                                                                                          | F-10           |
| T-39 | 異常系       | panic value が string や任意の値の場合          | `fmt.Sprintf("%v", v)` で文字列化してログに記録                                                                                  | F-10           |

### `internal/middleware/access_log.go` のテスト (T-40〜T-46)

| #    | カテゴリ     | 観点                                                         | 期待                                                                                                                                        | 関連          |
| ---- | ------------ | ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------- | ------------- |
| T-40 | 正常系       | 200 OK の handler                                            | 終了時に 1 行 INFO ログ、フィールド: `time` / `level=INFO` / `msg=access` / `request_id` / `method` / `path` / `status=200` / `duration_ms` | F-3, F-11, D3 |
| T-41 | 準正常系     | 4xx を返す handler                                           | level=WARN で出力                                                                                                                           | F-11, Q7      |
| T-42 | 異常系       | 5xx を返す handler                                           | level=ERROR で出力                                                                                                                          | F-11, Q7      |
| T-43 | 異常系       | recover が 500 を書いた場合                                  | access_log は最終 status=500 / level=ERROR を観測 (D1 順序の検証)                                                                           | D1            |
| T-44 | 正常系       | `duration_ms` の精度                                         | float64 で記録、ms 単位                                                                                                                     | F-3           |
| T-45 | セキュリティ | `path` のクエリ文字列に redact 対象キー (例: `?code=secret`) | `code` 値が `[REDACTED]` 化されてログに残る                                                                                                 | F-13, Q8      |
| T-46 | 正常系       | 開始時はログを出さない (D3: 終了時のみ)                      | 開始時に出力されるログが存在しない                                                                                                          | D3            |

### 統合テスト (T-53〜T-57)

| #    | カテゴリ     | 観点                                                                       | 期待                                                                            | 関連          |
| ---- | ------------ | -------------------------------------------------------------------------- | ------------------------------------------------------------------------------- | ------------- |
| T-53 | 正常系       | middleware チェーン全体を組み立てて `GET /health` を叩く                   | 200 OK + JSON `{"status":"ok"}` (M0.1 互換) + ヘッダ `X-Request-Id`             | F-5, F-15     |
| T-54 | 正常系       | 統合チェーン経由でログ buffer を取得し、access_log 行が 1 行出力されている | T-40 のフィールド構成と一致                                                     | F-3, D1, D3   |
| T-55 | 異常系       | テスト用 panic endpoint を組み込んで叩く                                   | T-33〜T-36 の挙動を統合チェーンで再現                                           | F-9, F-10, D1 |
| T-56 | セキュリティ | クライアント `Authorization: Bearer xxx` ヘッダ付きで叩く                  | アクセスログで `Authorization` が `[REDACTED]` 化される                         | F-13          |
| T-57 | セキュリティ | クライアント `X-Request-Id` に改行入りで叩く                               | レスポンスヘッダに新規生成された UUID v7、ログに `client_request_id` として残る | F-6           |

### `cmd/core/main.go` (T-63〜T-65)

| #    | カテゴリ | 観点                                       | 期待                                                                                                 | 関連          |
| ---- | -------- | ------------------------------------------ | ---------------------------------------------------------------------------------------------------- | ------------- |
| T-63 | 契約     | `grep -rn 'log\.Fatal' core/` 実行         | 出力が 0 件                                                                                          | F-12          |
| T-64 | 異常系   | 設定読み込み失敗時の挙動                   | 構造化ロガーで `Error` ログ + `os.Exit(1)`、`log.Fatalf` を使用しない                                | F-12          |
| T-65 | 正常系   | サーバー起動時の `event_id` 付き INFO ログ | `msg="core サーバーを起動します"` (日本語) / `event_id` (UUID v7 形式) / `addr` または `port` を含む | F-4, F-14, Q3 |

## Codex レビューコマンド (各ステップで使用)

```bash
CONTEXT_DIR="docs/context"

codex exec --sandbox read-only "Review the staged + unstaged Go diff in core/internal/{middleware,server,health} and core/cmd/core/.

Context to read first:
- ${CONTEXT_DIR}/app/architecture.md
- ${CONTEXT_DIR}/backend/conventions.md
- ${CONTEXT_DIR}/backend/patterns.md
- docs/specs/7/index.md (本フェーズ設計書)
- docs/specs/7/prompts/P2_01_core_middleware_server_main.md (本プロンプト)

Then review the current diff (use git diff). Check:
1) TDD compliance: テストが先行コミットされているか / 各 T-N がカバーされているか
2) D1 middleware 順序: request_id (最外側) → access_log → recover → handler となっているか
3) X-Request-Id レスポンスヘッダが next 呼び出し前に先行設定されているか (handler の WriteHeader 後の Set は無視される)
4) panic 時に stack trace が HTTP レスポンスに混入していないか (内部ログのみ)
5) UUID v7 不使用がないか (NewV7 のみ、NewRandom 禁止)
6) log.Fatal 全廃: grep -rn 'log\\.Fatal' core/ が 0 件か、log.Fatal を使っていないか
7) M0.1 の /health 外形互換 (200 OK + status:ok JSON) が維持されているか
8) 起動時ログに event_id (UUID v7) と日本語メッセージが含まれるか
9) シークレット安全性: redact が機能しているか (Authorization ヘッダ、code クエリ等)
10) net/http の慣用句: ResponseWriter のラップ、context 伝播、defer の正しい使い方

Report issues as CRITICAL/HIGH/MEDIUM/LOW in Japanese with file:line references."
```

## 完了条件

- [ ] Issue #12 (P1) のマージ済みを確認
- [ ] 作業ブランチ `feature/m0.2-impl-middleware-server` を作成
- [ ] `core/internal/middleware/request_id.go` 実装 + `request_id_test.go` で T-22〜T-32 (11 ケース) pass
- [ ] `core/internal/middleware/access_log.go` 実装 + `access_log_test.go` で T-40〜T-46 (7 ケース) pass
- [ ] `core/internal/middleware/recover.go` 実装 + `recover_test.go` で T-33〜T-39 (7 ケース) pass
- [ ] `core/internal/server/server.go` 修正: D1 順序で middleware チェーン組込
- [ ] `core/internal/server/server_test.go` (または `integration_test.go`) で T-53〜T-57 (5 ケース) pass
- [ ] `core/cmd/core/main.go` 修正: `log.Fatal` 全廃、構造化ロガー初期化、起動 INFO ログ + `event_id`
- [ ] `core/cmd/core/main_test.go` で T-63〜T-65 (3 ケース) pass
- [ ] `core/internal/health/health.go` の暫定コメント解消
- [ ] `grep -rn "log\.Fatal" core/` の出力が **0 件**
- [ ] `make -C core build && make -C core test && make -C core lint` 全件 pass
- [ ] M0.1 の `/health` 外形互換 (HTTP 200 + `{"status":"ok"}` + Content-Type) を維持
- [ ] 各ステップで Codex レビューを実施、CRITICAL=0 / HIGH=0 / MEDIUM<3
- [ ] PR 作成 (`gh pr create` with `--assignee` + `--label "種別:実装" --label "対象:基盤" --label "対象:サーバー"`)
- [ ] `/pr-codex-review {番号}` でゲート通過
- [ ] PR の Test plan を実機確認して `[x]` に書き換え
- [ ] PR をマージ (Issue #13 が自動 close)
- [ ] 親 Issue #7 の task list で #13 にチェックが付くことを確認
