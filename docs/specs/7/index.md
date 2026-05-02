# 設計 #7: ログ・エラー・request_id の規約と core/ への組み込み

- 関連要求: [`docs/requirements/7/index.md`](../../requirements/7/index.md)
- 関連 Issue: [#7](https://github.com/mktkhr/id-core/issues/7)
- マイルストーン: [M0.2: ログ・エラー・request_id の規約確定](https://github.com/mktkhr/id-core/milestone/2)
- 状態: 着手中
- 起票日: 2026-05-02
- 最終更新: 2026-05-02

## 関連資料

- 要求文書: [`docs/requirements/7/index.md`](../../requirements/7/index.md)
- 認可マトリクス (正本): [`docs/context/authorization/matrix.md`](../../context/authorization/matrix.md) (本スコープは認可対象外)
- 先行設計書: [`docs/specs/1/index.md`](../1/index.md) (M0.1: core/ の最小 HTTP サーバー)
- アーキテクチャ概要: [`docs/context/app/architecture.md`](../../context/app/architecture.md)
- backend 規約: [`docs/context/backend/conventions.md`](../../context/backend/conventions.md), [`docs/context/backend/patterns.md`](../../context/backend/patterns.md), [`docs/context/backend/registry.md`](../../context/backend/registry.md)
- 関連スキル: `/backend-logging` (ログ規約), `/backend-security` (シークレット redact), `/backend-architecture` (Domain 層は副作用なし)
- 関連 ADR: なし (本スコープでの新規 ADR は論点解決時に判断。F-7 の基本形を逸脱する場合は要起票)

## 要件の解釈

`core/` (id-core 本体) に**ログ・エラー・request_id の横断規約と middleware**を導入する設計。M0.1 では `/health` の最小骨格しか持たなかった `core/` に、以降のマイルストーン (M0.3 DB / M1.x OIDC) が乗る基盤層を追加する。

新規エンドポイントは追加せず、既存 `/health` の挙動は外形互換 (200 OK + `{"status":"ok"}`) のまま、ログ規約と middleware に乗せ替える。

要求 F-1〜F-17 を以下のように分解する:

| 要求                                          | 設計対応                                                                                                                                                                                                                                  |
| --------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| F-1 (JSON Lines + ログインジェクション対策)   | `internal/logger/` に `log/slog` ベースの構造化ロガーを実装。`slog.NewJSONHandler` で JSON エンコーダ経由出力、改行・制御文字は自動エスケープ                                                                                             |
| F-2 (フォーマット切替)                        | 環境変数 `CORE_LOG_FORMAT=json\|text` で切替。デフォルト `json`、開発時は `text` で `slog.NewTextHandler` を使用                                                                                                                          |
| F-3 (HTTP 経路ログのフィールド)               | `internal/middleware/access_log.go` でリクエスト終了時にアクセスログ 1 行出力 (D3)。`time` は RFC3339Nano UTC (Q4)                                                                                                                        |
| F-4 (非 HTTP 経路の `event_id`)               | `internal/logger/` に `WithEventID(ctx, id)` のヘルパーを置き、起動 / signal handler / 将来のジョブが利用。生成は UUID v7 (Q3)                                                                                                            |
| F-5 (`request_id` 生成 + context 伝播)        | `internal/middleware/request_id.go` で UUID v7 (Q3) を生成し `context.Context` に格納、レスポンスヘッダ `X-Request-Id` を必ず返す                                                                                                         |
| F-6 (クライアント `X-Request-Id` 妥当性検証)  | 上記 middleware で長さ ≤128、文字種 `0x21`–`0x7E` の検証。不正値は破棄して再生成、ログには `client_request_id` (サニタイズ後) として残す                                                                                                  |
| F-7 (内部 API エラーレスポンス JSON)          | `internal/apperror/` に基本形 `{ code, message, details?, request_id }` の型と JSON シリアライザを実装 (`backend-architecture` 共通インフラ規約に整合)。Q5 で最終形式確定                                                                 |
| F-8 (OIDC エラー方針)                         | 設計書として M1.x の OIDC エンドポイントが守るべき RFC 6749 / 6750 / OIDC Core 準拠規約を明文化。本スコープでは実装しない                                                                                                                 |
| F-9 (3 系統エラーハンドラ)                    | `internal/middleware/recover_and_error.go` で panic / 既知エラー / 未捕捉 error を統一処理。`request_id` を必ずレスポンスへ含める                                                                                                         |
| F-10 (panic スタックトレース外部漏洩防止)     | エラーハンドラ middleware は panic 時に固定メッセージ + `request_id` のみ返す。スタックトレースは内部の構造化ログ (level=ERROR) にのみ記録                                                                                                |
| F-11 (ログレベル: 5xx/panic=ERROR、4xx≤WARN)  | エラーハンドラ middleware が status から自動判定 (5xx/panic=ERROR、4xx=WARN、それ以外=INFO)。詳細ガイドは `backend-logging` の表 (Q7 で採用) に従う                                                                                       |
| F-12 (`log.Fatal*` 全廃)                      | `core/cmd/core/main.go` の `log.Fatalf` を構造化ロガー (`log/slog` ベース) の `Error` ログ出力 + `os.Exit(1)` 終了処理に置換。CI で `grep -rn "log\.Fatal" core/` を実行 (F-16 と連携)                                                    |
| F-13 (シークレット redact)                    | `internal/logger/redact.go` に deny-list redactor を実装。HTTP middleware からヘッダ・body・query を逐次 redact、エラー `details` も再帰走査                                                                                              |
| F-14 (Domain 層ログ禁止)                      | Domain 層 (今回は `internal/health/` 等の handler 直下のみ存在) はロガーを直接呼ばない。Handler / middleware が `context.Context` から `request_id` / `event_id` を取得して付与                                                           |
| F-15 (`make build && make test && make lint`) | 既存 Makefile を拡張 (新規 target は不要、既存 target 内で動作する)                                                                                                                                                                       |
| F-16 (ログスキーマ契約テスト)                 | `internal/logger/contract_test.go` で HTTP 系・非 HTTP 系の 2 系統に分割。lib/snapshot 系のフィールド存在 + 型検証                                                                                                                        |
| F-17 (規約書)                                 | `docs/context/backend/conventions.md` のロギング・テレメトリ節を本スコープで詳細化 + エラーハンドリング節を新設 (Q10)。最低必須項目: ログフィールド定義 / エラー境界 / redact 一覧 / レベルガイド / 運用手順。検証は `/doc-review` + F-16 |

## 設計時の論点

要求文書から引き継いだ論点 Q1〜Q10。さらに設計フェーズで判明した内部論点 D1〜D3 を追加。決定責任者は全件 mktkhr、期限は 2026-05-16 (要求文書と整合)。

| #   | 論点                                                            | 候補                                                                                                                                                           | 決定                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | 理由                                                                                                                                                                                                                                                                                                                                                                                                                        |
| --- | --------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Q1  | ロガー実装                                                      | (a) `log/slog` 標準 (b) `zap` (c) `zerolog`                                                                                                                    | **(a) `log/slog` (Go 標準)**                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | `context/backend/conventions.md` で「M0.2 で `log/slog` ベース」と明記、`backend-logging` でも第一推奨。標準ライブラリで依存追加なし、API 安定性が高い                                                                                                                                                                                                                                                                      |
| Q2  | ログフォーマット切替の環境変数名・値                            | (a) `CORE_LOG_FORMAT=json\|text` (b) `CORE_ENV=production\|development` 間接決定 (c) その他                                                                    | **(a) `CORE_LOG_FORMAT=json\|text`** (デフォルト `json`)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | 既存規約 `CORE_<NAME>` プレフィックスと整合 (`CORE_PORT` と並ぶ)、12-factor のログ出力先指定原則 (環境変数で挙動を切替)                                                                                                                                                                                                                                                                                                     |
| Q3  | 一意 ID の生成方式 (`request_id` / `event_id` 共通)             | (a) UUID v4 (b) UUID v7 (c) ULID (d) snowflake 系                                                                                                              | **(b) UUID v7**                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | 時系列ソート可 (Unix ms 先頭 48 bit)、B-tree 主キーとしてのインデックス効率 (M0.3 以降の DB 主キーでも統一)、推測困難性は v7 でも 74 bit 確保。**プロジェクトポリシーで v4 は使用禁止**                                                                                                                                                                                                                                     |
| Q4  | `time` フィールドのフォーマット                                 | (a) RFC3339Nano (UTC、`Z` suffix) (b) RFC3339 秒精度 (c) Unix epoch (ms)                                                                                       | **(a) RFC3339Nano (UTC、`Z` suffix 強制)**                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | `log/slog` のデフォルトフォーマット、ナノ秒精度は診断・順序付けに有用、UTC で TZ 混在排除。PostgreSQL `timestamptz` (UTC 内部表現、μs 精度) と完全互換。**実装方針**: プロセス全体に副作用を持つ `time.Local = time.UTC` は使わず、`slog.NewJSONHandler` の `ReplaceAttr` フックで `time.Time` を `t.UTC().Format(time.RFC3339Nano)` に変換するか、`internal/logger/` の公開 API 内で UTC 化を明示                          |
| Q5  | 内部 API エラーレスポンス JSON 形式の最終形                     | (a) F-7 基本形 (b) RFC 7807 problem+json (c) F-7 基本形 + RFC 7807 互換フィールド                                                                              | **(a) F-7 基本形 (`{ code, message, details?, request_id }`) を `internal/apperror/` パッケージで実装**                                                                                                                                                                                                                                                                                                                                                                                                                                                           | `context/backend/patterns.md` で「M0.2 で `apperror` パッケージ導入」明記。RFC 7807 はクライアント側の利点が薄く、フロントエンド (examples) からの取り回しを優先。Content-Type は `application/json; charset=utf-8` 固定                                                                                                                                                                                                    |
| Q6  | OIDC エラーレスポンスでの `request_id` 露出方法                 | (a) `error_uri` 埋め込み (b) RFC 仕様外フィールド `request_id` (c) ヘッダ `X-Request-Id` のみ                                                                  | **(c) ヘッダ `X-Request-Id` のみ**                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | RFC 6749 / 6750 完全準拠 (ボディ仕様改変なし)、`X-Request-Id` は全エンドポイント共通で既に必須 (F-5)、クライアントは header から拾える。`error_uri` 埋め込みは仕様用途と意味的に異なる                                                                                                                                                                                                                                      |
| Q7  | ログレベル使い分けガイド詳細                                    | DEBUG / INFO / WARN / ERROR の境界をどう書くか                                                                                                                 | **`backend-logging` スキルの表をそのまま採用**: DEBUG (本番無効、開発・調査用) / INFO (業務イベント、リクエスト成功・開始/終了) / WARN (4xx エラー、想定外だが処理継続可能) / ERROR (5xx・panic・DB 障害・予期しないエラー、処理失敗)                                                                                                                                                                                                                                                                                                                             | F-11 (5xx/panic=ERROR、4xx≤WARN) と完全に整合、規約変更不要、後続マイルストーンで再定義不要                                                                                                                                                                                                                                                                                                                                 |
| Q8  | redact 対象キーの完全一覧                                       | F-13 の最低リスト + α                                                                                                                                          | **以下を完全リストとして固定** (M0.2 確定):<br>**ヘッダ** (case-insensitive): `Authorization`, `Cookie`, `Set-Cookie`, `Proxy-Authorization`, `X-Api-Key`, `X-Auth-Token`<br>**body / query / form** (case-insensitive 完全一致、ネスト・配列再帰走査): `password`, `current_password`, `new_password`, `access_token`, `refresh_token`, `id_token`, `code`, `code_verifier`, `client_secret`, `assertion`, `client_assertion`, `private_key`, `secret`, `api_key`, `jwt`, `bearer_token`<br>**redact 値**: 文字列 `[REDACTED]` 固定 (長さ・存在情報を漏らさない) | `backend-logging` の OIDC 注意事項 (ID/access/refresh トークン全文禁止、`code_verifier` 禁止) と OAuth 2.0 / OIDC RFC 6749/6750 のトークン項目を全網羅。M1.x 以降の OIDC 実装で漏れがないようここで先に固定                                                                                                                                                                                                                 |
| Q9  | ログ出力失敗時のフォールバック先                                | (a) 標準エラー (b) ring buffer (c) 単に drop                                                                                                                   | **(a) 標準エラー (stderr) フォールバック + drop カウンタ (atomic)**                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | NFR 可用性「リクエスト継続優先」に整合。stderr に最低限の構造化ログを書き、drop 件数は atomic counter で内部保持。drop カウンタの外部公開 (メトリクス露出) は M1.x のメトリクス導入時に対応                                                                                                                                                                                                                                 |
| Q10 | 規約書の格納場所                                                | (a) `core/docs/conventions/logging.md` (b) `docs/context/conventions/` (c) `docs/specs/7/` 内に閉じる (d) 既存 `docs/context/backend/conventions.md` の拡張    | **(d) `docs/context/backend/conventions.md` のロギング・テレメトリ節を本スコープで詳細化 + エラーハンドリング節を新設して `apperror` 規約を記載**                                                                                                                                                                                                                                                                                                                                                                                                                 | 既存規約整備パスに乗る、M0.3 / M1.x 以降からも単一 SoT として参照しやすい、`core/docs/` の新ディレクトリ作成を回避                                                                                                                                                                                                                                                                                                          |
| D1  | middleware の構成順序 (recover / request_id / access_log)       | (a) recover → request_id → access_log → handler (b) request_id → recover → access_log → handler (c) **request_id → access_log → recover → handler** (d) その他 | **(c) `request_id` → `access_log` → `recover` → `handler`**                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | (1) `request_id` を最外側に置くことで panic 時を含む全ログレコードに `request_id` が付く。(2) `recover` を `access_log` の**内側**に置くことで、handler の panic は `recover` が捕捉して 500 応答に変換されてから `access_log` の `defer` に戻る。これにより `access_log` は最終 status (500) と level (ERROR) を正しく観測できる。逆順 (b) だと `access_log.defer` が panic unwind 中に走り、status=0 / level 誤判定になる |
| D2  | `internal/logger/` の公開 API                                   | (a) `slog.Logger` を直接 export (b) 薄い独自インターフェース (c) handler/middleware/service ごとに wrapper 関数を提供                                          | **(b) 薄い独自インターフェース** (`logger.Info(ctx, msg, ...args)` / `logger.Error(ctx, msg, err, ...args)` 形式)                                                                                                                                                                                                                                                                                                                                                                                                                                                 | `backend-logging` のコード例と整合、Q1 の `slog.Logger` を内部実装として隠蔽し将来の差し替え (zap 等) に備える、context.Context から `request_id` / `event_id` を自動付与する責務を集約できる                                                                                                                                                                                                                               |
| D3  | アクセスログの出力タイミング (リクエスト開始時 / 終了時 / 両方) | (a) 終了時のみ (status / duration を含めて 1 行) (b) 開始時 + 終了時 (c) 終了時 + エラー時のみ別エントリ                                                       | **(a) 終了時のみ 1 行**                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | `backend-logging` 規約・ログ集約基盤との互換性、grep が単純化、ログ量を抑制。長時間処理の進捗観測は別途 INFO ログで対応する想定 (M1.x 以降)                                                                                                                                                                                                                                                                                 |

## 実装対象

| モジュール                         |    実装有無     |
| ---------------------------------- | :-------------: |
| `core/`                            |       ✅        |
| `examples/go-react/backend/`       |        —        |
| `examples/go-react/frontend/`      |        —        |
| `examples/kotlin-nextjs/backend/`  |        —        |
| `examples/kotlin-nextjs/frontend/` |        —        |
| データベース                       | — (M0.3 で対応) |

## ディレクトリ構成

M0.1 で確立した構成に新規パッケージを追加する。

```
core/
├── go.mod
├── Makefile
├── README.md
├── .gitignore
├── cmd/
│   └── core/
│       └── main.go              # 構造化ロガー初期化 + サーバー起動 (log.Fatal 全廃)
├── internal/
│   ├── config/
│   │   ├── config.go            # 既存 + ロガー設定 (Q2 の env を追加)
│   │   └── config_test.go
│   ├── logger/                  # ★ 新規
│   │   ├── logger.go            # log/slog ベース構造化ロガーの初期化、公開 API (D2 薄い独自 IF: Info/Warn/Error/Debug)
│   │   ├── context.go           # context.Context への request_id (UUID v7) / event_id 付与・取得ヘルパー
│   │   ├── format.go            # CORE_LOG_FORMAT=json|text 切替 (Q2)、time は RFC3339Nano UTC 強制 (Q4)
│   │   ├── redact.go            # F-13 deny-list redactor (Q8 完全リスト)
│   │   ├── redact_test.go       # redact 単体テスト
│   │   ├── fallback.go          # ログ出力失敗時の stderr フォールバック + drop counter (Q9)
│   │   └── contract_test.go     # F-16 ログスキーマ契約テスト (HTTP 系 + 非 HTTP 系)
│   ├── apperror/                # ★ 新規 (Q5 で apperror パッケージ採用)
│   │   ├── apperror.go          # 既知エラー型 (CodedError) と code/message 変換、error chain ラップ
│   │   ├── response.go          # F-7 基本形 (`{ code, message, details?, request_id }`) の JSON レスポンス生成
│   │   └── apperror_test.go
│   ├── middleware/              # ★ 新規
│   │   ├── request_id.go        # F-5/F-6: 生成 / 検証 / context 伝播 / レスポンスヘッダ
│   │   ├── request_id_test.go
│   │   ├── recover.go           # F-9/F-10: panic 捕捉 + 構造化ログ + 固定メッセージ応答
│   │   ├── recover_test.go
│   │   ├── access_log.go        # F-3: アクセスログ出力 (status / duration_ms 等)
│   │   └── access_log_test.go
│   ├── server/
│   │   └── server.go            # ServeMux + middleware チェーン (D1: request_id → access_log → recover → handler)
│   └── health/
│       ├── health.go            # 既存。挙動互換 (中身は変更最小)
│       └── health_test.go
└── bin/
    └── core
```

## DB 設計

**スコープ外** (M0.3 で対応)。

## API 設計

### エンドポイント一覧 (M0.2 で新規追加するものなし)

| Method | Path      | 認証        | 変更内容                                                                         |
| ------ | --------- | ----------- | -------------------------------------------------------------------------------- |
| GET    | `/health` | 不要 (公開) | 外形互換。middleware チェーン (request_id / access_log / recover) を通る変更のみ |

### middleware チェーン仕様

D1 で決定した順序: **`request_id` (最外側) → `access_log` → `recover` → `handler`**。

リクエスト処理フロー:

```
Client → request_id (生成 / 検証 / context 注入) → access_log (status/duration 計測) → recover (panic 捕捉) → handler
                                                  ↑                                       ↑
                                              終了時にここで 1 行出力                   panic 時はここで
                                              (recover が 500 を書いた後)              ERROR ログ + 500 応答
```

順序の重要性: `access_log` を `recover` の**外側**に置くことで、handler が panic しても `recover` が先に 500 応答を書き、その後 `access_log` の `defer` が status=500 / level=ERROR を正しく記録できる。逆順では `access_log.defer` が panic unwind 中に走るため status=0 / level 誤判定が発生する。

各 middleware の責務:

| middleware   | 責務                                                                                                                                                                                                                               |
| ------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `request_id` | クライアント `X-Request-Id` の妥当性検証 (F-6)、不正なら破棄して再生成 (UUID v7、Q3)、`context.Context` に格納、レスポンスヘッダに常に付与                                                                                         |
| `recover`    | `defer recover()` で panic 捕捉、内部ログに stack trace 含む ERROR 記録 (F-10)、クライアントには固定メッセージ + `request_id` の F-7 基本形 JSON で 500 を返す                                                                     |
| `access_log` | リクエスト終了時に 1 行出力 (D3): `time` (RFC3339Nano UTC、Q4) / `level` / `msg=access` / `request_id` / `method` / `path` / `status` / `duration_ms`。レベルは F-11 / Q7 ルール (5xx=ERROR / 4xx=WARN / それ以外=INFO) で自動判定 |

### エラーレスポンス JSON 規約

#### 内部 API (F-7)

基本形:

```json
{
  "code": "INVALID_PARAMETER",
  "message": "human readable message",
  "details": {},
  "request_id": "<uuid>"
}
```

- `code` は `SCREAMING_SNAKE_CASE` の固定文字列 (機械可読)
- `message` は人間可読 (i18n は M3.3 以降)
- `details` は object / array に限定、シークレット禁止 (F-13 の redact が再帰走査)
- `request_id` は middleware で付与した値を必ず含める
- `Content-Type: application/json; charset=utf-8`

#### OIDC エンドポイント (F-8、本スコープでは方針宣言のみ)

M1.x 以降の `/authorize` / `/token` / `/userinfo` 等は RFC 6749 / 6750 / OIDC Core 準拠:

```json
{ "error": "invalid_request", "error_description": "...", "error_uri": "..." }
```

Q6 で決定: **OIDC エンドポイントでの `request_id` 露出はレスポンスヘッダ `X-Request-Id` のみ**。RFC 6749 / 6750 のボディ仕様 (`error` / `error_description` / `error_uri` の 3 フィールド) は改変しない。クライアントは header から `X-Request-Id` を取得する。F-7 (内部 API) と F-8 (OIDC) はエンドポイント単位で固定し、混在禁止。

### ログ規約

#### ログメッセージの言語

`backend-logging` 規約に従い、すべてのログメッセージ (`msg` フィールド) は**日本語で記載する**。アクセスログ系の `msg` (`access` 等の固定文字列) は識別子として扱うため英数字でよいが、業務イベント・エラーログは日本語にする。

#### HTTP 経路 (F-3)

```json
{
  "time": "2026-05-02T01:00:00.123456789Z",
  "level": "INFO",
  "msg": "access",
  "request_id": "01978f86-3a01-7b0c-9f3e-1d8c4a9b6e2f",
  "method": "GET",
  "path": "/health",
  "status": 200,
  "duration_ms": 1.23
}
```

- `time`: RFC3339Nano UTC、`Z` suffix 強制 (Q4)。`slog` ハンドラの `ReplaceAttr` で `t.UTC().Format(time.RFC3339Nano)` に変換 (プロセス全体への副作用となる `time.Local = time.UTC` は使わない)
- `request_id`: UUID v7 文字列形式 (Q3)。先頭 48bit が Unix ms timestamp で時系列ソート可能
- 追加フィールドは設計フェーズの最終整理 (`/spec-track`) または運用開始後に拡張する想定

#### 非 HTTP 経路 (F-4)

起動・signal handler・将来のジョブが出すログには `event_id` を必須付与:

```json
{
  "time": "2026-05-02T01:00:00.000000000Z",
  "level": "INFO",
  "msg": "core サーバーを起動します",
  "event_id": "01978f86-3a01-7b0c-9f3e-1d8c4a9b6e2f",
  "port": 8080
}
```

`event_id` は起動毎・ジョブ毎に発番される一意 ID。生成方式は `request_id` と共通で **UUID v7** (Q3)。

### redact 規約 (F-13)

deny-list:

Q8 で確定した完全一覧:

| 適用面                  | 対象キー                                                                                                                                                                                                                          | 照合規則                                             |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------- |
| HTTP リクエストヘッダ   | `Authorization`, `Cookie`, `Set-Cookie`, `Proxy-Authorization`, `X-Api-Key`, `X-Auth-Token`                                                                                                                                       | case-insensitive                                     |
| JSON body / `details`   | `password`, `current_password`, `new_password`, `access_token`, `refresh_token`, `id_token`, `code`, `code_verifier`, `client_secret`, `assertion`, `client_assertion`, `private_key`, `secret`, `api_key`, `jwt`, `bearer_token` | case-insensitive かつ完全一致 / ネスト・配列再帰走査 |
| query / form パラメータ | 同上 (body と同一リスト)                                                                                                                                                                                                          | case-insensitive かつ完全一致                        |
| error chain メッセージ  | 同上 (文字列の fuzzy 検出は行わず、構造化フィールド経由でのみ redact)                                                                                                                                                             | キー一致のみ (本文の string match はしない)          |

**redact 値**: 文字列 `[REDACTED]` 固定で置換する。長さや存在情報を漏らさない (`****` のようなマスキングではなく `[REDACTED]` とする理由: 攻撃者にトークン長を推測させない)。

> `backend-logging` の OIDC 注意事項に準拠: ID/access/refresh トークン全文ログ禁止 (jti / sub / 先頭数文字のみが必要なら個別に明示出力)。`code_verifier` (PKCE) は値そのものが秘匿対象。

### ログ出力失敗時の挙動 (Q9)

ロガーの書き込みが失敗した場合 (例: stdout writer エラー、stdout 詰まり):

1. リクエスト処理は**継続する** (NFR 可用性最優先)
2. 失敗したログレコードを **stderr に最低限のフォールバック出力** を試みる (stderr も失敗した場合は静かに drop)
3. drop 件数を **atomic counter** で内部保持 (`expvar` または `internal/logger/fallback.go` 内の sync/atomic)
4. drop counter の外部公開 (Prometheus / OpenTelemetry メトリクス) は M1.x のメトリクス導入時に対応 (本スコープ外)

部分一致は誤検知の温床なので**禁止**。完全一覧は Q8 で確定。

## 認可設計

**本スコープは認可対象外**。

[`docs/context/authorization/matrix.md`](../../context/authorization/matrix.md) (正本) と差分なし。新規エンドポイント追加なし、`/health` は引き続き認証不要・公開エンドポイント。

> マスターと差分なし。本スコープでの認可マトリクス変更提案もなし。

## フロー図 / シーケンス図

`/spec-diagrams` で生成予定。最低限以下の 3 図を想定:

1. リクエスト処理シーケンス (request_id 生成 → middleware チェーン → handler → access_log)
2. panic 時のフロー (recover → 内部 ERROR ログ → 固定メッセージ応答)
3. 不正な `X-Request-Id` 受信時のフロー (検証 → 破棄 → 再生成 → `client_request_id` をログに残す)

## テスト観点

`/spec-tests` で詳細化予定。最低限以下のカテゴリを想定:

### バックエンド単体テスト

- `internal/logger/`: redact (deny-list / case-insensitive / ネスト / 配列)、context 伝播、JSON エンコード時のインジェクション対策
- `internal/middleware/request_id.go`: 妥当な値の採用 / 不正値の再生成 / レスポンスヘッダ常時付与 / context への伝播
- `internal/middleware/recover.go`: panic 捕捉 / 固定メッセージ応答 / スタックトレース外部非露出 / 内部ログに stack 記録
- `internal/middleware/access_log.go`: status による level 自動判定 (5xx=ERROR / 4xx=WARN)、`duration_ms` の記録
- `internal/apperror/`: F-7 基本形シリアライズ / `details` のシークレット redact

### 統合テスト

- `httptest` で `/health` を叩き、レスポンスヘッダに `X-Request-Id` が必ず付くことを確認
- panic 経路 (テスト用 endpoint または `recover` 単体テスト) で 500 + 固定メッセージ + `request_id` を確認
- ログ出力を buffer に取得し、F-3 のフィールドが揃うこと、redact 対象が `[REDACTED]` 化されていることを確認

### F-16 契約テスト

- HTTP 系 1 ケース、非 HTTP 系 1 ケース。フィールド存在 + 型を検証 (将来のフィールド追加は許容、削除はテスト失敗)

### E2E

M0.2 では実施しない (M1.5 から)。

## 既存資料からの差分

`docs/context/` への影響:

- `docs/context/backend/conventions.md`: ログ規約 / エラーレスポンス規約 / request_id 伝播の小節を追加 (Q10 の決定次第で本ファイルへ書くか別ファイルへ分離か変動)
- `docs/context/backend/patterns.md`: middleware チェーン / context への ID 付与パターン / redact の最小サンプル
- `docs/context/backend/registry.md`: `core/internal/{logger,apperror,middleware}` の各パッケージを登録

その他:

- `core/cmd/core/main.go` の `log.Fatalf` 暫定実装を構造化ロガーに置換 (F-12 grep 完了条件)
- `core/Makefile` に lint で `grep -rn "log\.Fatal" core/` を含めるか検討 (D に追加するか実装プロンプト側で扱うか確定)
- `core/README.md` にログ・エラー規約の入口段落を追加し、規約書 (Q10) へリンク

## 設計フェーズ状況

| フェーズ               | 状態       | 備考                                                                                                                                                                                                                                          |
| ---------------------- | ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1. 起票                | 完了       | 2026-05-02                                                                                                                                                                                                                                    |
| 2. 下書き              | 完了       | テンプレートから生成、要件 F-1〜F-17 を構造化、論点 Q1〜Q10 + D1〜D3 を整理                                                                                                                                                                   |
| 3. 規約確認            | 完了       | `/check-convention` で context/backend (conventions/patterns/registry) と backend-logging / backend-architecture / backend-security スキルを調査。`internal/apperror/` 命名規約 / `code_verifier` redact / 日本語ログメッセージを設計書に反映 |
| 4. 論点解決            | 完了       | `/spec-resolve` で Q1〜Q10 + D1〜D3 を全件確定 (規約由来 6 件 + ユーザー判断 7 件)                                                                                                                                                            |
| 5. DB 設計             | スコープ外 | M0.3 で対応                                                                                                                                                                                                                                   |
| 6. API 設計            | 完了       | エラーレスポンス規約 (内部 API + OIDC 方針) / ログ規約 (HTTP + 非 HTTP) / redact 規約 (Q8 完全一覧) / middleware チェーン (D1 順序) を確定                                                                                                    |
| 7. 認可設計            | 完了       | 認可スコープ外、マスター差分なし                                                                                                                                                                                                              |
| 8. 図                  | 未着手     | `/spec-diagrams` で生成 (3 図想定)                                                                                                                                                                                                            |
| 9. テスト設計          | 未着手     | `/spec-tests` で T-N 番付与                                                                                                                                                                                                                   |
| 10. 差分整理           | 進行中     | Q10 で `docs/context/backend/conventions.md` 拡張 + エラーハンドリング節新設に確定。`/spec-track` で最終整理                                                                                                                                  |
| 11. 実装プロンプト生成 | 未着手     | `/spec-prompts` または `/issue-from-spec`                                                                                                                                                                                                     |

## 変更履歴

| 日付       | 変更内容                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 2026-05-02 | 起票 (要求 #7 から `/spec-create` で生成)。F-1〜F-17 の設計対応マッピング、論点 Q1〜Q10 + D1〜D3 を整理                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| 2026-05-02 | `/check-convention` 実施: `internal/errors/` → `internal/apperror/` にリネーム (backend-architecture 共通インフラ規約整合)、F-13 redact 対象に `code_verifier` 追加 (backend-logging OIDC 注意事項)、ログ規約節に「ログメッセージは日本語」原則を追記 (backend-logging)。フェーズ 3 を完了に更新                                                                                                                                                                                                                                           |
| 2026-05-02 | `/spec-resolve` 実施: 論点 Q1〜Q10 + D1〜D3 を全件確定。Q1=`log/slog`、Q2=`CORE_LOG_FORMAT=json\|text`、Q3=**UUID v7** (v4 禁止)、Q4=RFC3339Nano UTC、Q5=F-7 基本形 + `internal/apperror/` パッケージ、Q6=`X-Request-Id` ヘッダのみ、Q7=`backend-logging` のレベル表、Q8=redact 完全一覧 (16 キー + 6 ヘッダ)、Q9=stderr フォールバック + drop カウンタ、Q10=`docs/context/backend/conventions.md` 拡張、D1=`request_id`→`recover`→`access_log`→`handler`、D2=薄い独自 IF、D3=終了時 1 行。フェーズ 4 / 6 を完了、フェーズ 10 の方向性確定 |
| 2026-05-02 | Codex PR レビュー (PR #10) 指摘反映: D1 middleware 順序を `request_id`→`recover`→`access_log`→`handler` から **`request_id`→`access_log`→`recover`→`handler`** に変更 (panic 時に `access_log.defer` が status=0 を記録してしまう問題回避)。「既存資料からの差分」の `core/internal/{logger,errors,middleware}` 表記を `apperror` に統一。Q4 の実装方針に「プロセス全体副作用となる `time.Local = time.UTC` は使わず `slog` ハンドラの `ReplaceAttr` で UTC 変換」を明記                                                                   |
