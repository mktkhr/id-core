# バックエンド規約 (id-core / Go)

> 最終更新: 2026-05-02 (M0.3: DB 接続 + マイグレーション基盤の規約確定 反映)

## モジュール構成

- モジュール名: `github.com/mktkhr/id-core/core`
- ルート: `core/`
- Go 最低バージョン: `1.22+` (`net/http` の ServeMux メソッド・パスパターンを利用するため)
- 動作確認バージョン: `1.26.2`

## ディレクトリ規約

```
core/
├── cmd/<binary>/           # 実行ファイルのエントリポイント (1 binary = 1 ディレクトリ)
├── internal/<feature>/     # 機能パッケージ。外部公開しない
└── bin/                    # ビルド成果物 (.gitignore で除外)
```

`internal/` 配下のパッケージは Go の `internal` 規則によりモジュール外部からは import 不可。
後続マイルストーンで `internal/features/<feature>/{domain,application,infrastructure,presentation}` の DDD レイヤを導入予定 (`backend-architecture` スキル参照)。

## Makefile 規約

`core/Makefile` は以下のターゲットを最低限提供する:

| ターゲット         | 用途                                                                                                                                                 |
| ------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| `help`             | ターゲット一覧を表示 (`make` または `make help`)                                                                                                     |
| `build`            | バイナリをビルド (`go build -o bin/core ./cmd/core`)                                                                                                 |
| `run`              | ビルド + 起動                                                                                                                                        |
| `test`             | `go test -race ./...` (DB 不要なユニットテストのみ)                                                                                                  |
| `test-cover`       | カバレッジ計測 (`-coverprofile=coverage.txt`)                                                                                                        |
| `test-integration` | 統合テスト (DB 必要、build tag = `integration`、`-p 1` で package 単位順次実行)                                                                      |
| `lint`             | `go vet ./...` + プロジェクト固有禁止チェック (`log.Fatal*` を非テスト `.go` ファイルに新規追加すると lint failure + マイグレーション連番衝突検出)   |
| `clean`            | `bin/` と `coverage.txt` を削除                                                                                                                      |
| `migrate-*` 9 種   | マイグレーション CLI (install / create / up / up-one / down / down-all / force / version / status) — 詳細は本ファイル「DB / マイグレーション」節参照 |

`golangci-lint` の導入は後続マイルストーンで検討。

## 環境変数読み込みパターン

- 環境変数は `internal/config/config.go` の `Load()` で集約読み込み
- バリデーションエラーは `error` で返す (`log.Fatal` を直接呼ばない → テスト容易性確保)
- `cmd/<binary>/main.go` で `error` を受けて `logger.Error` + `os.Exit(1)` で異常終了 (`log.Fatal*` の使用は禁止 / Makefile lint で検査)
- 命名: `CORE_<NAME>` プレフィックス (例: `CORE_PORT`, `CORE_LOG_FORMAT`)
- 範囲制約があるものは `MinXxx` / `MaxXxx` 定数として `config` パッケージで宣言

環境変数の一覧は `docs/context/backend/registry.md` の「環境変数一覧」を参照。

## DB / マイグレーション

M0.3 で導入。OIDC / 認可コード / セッション / 認証情報の永続化基盤。実テーブル DDL は M2.x 以降で本格化、本マイルストーンでは接続層 + マイグレーション運用 + smoke table のみ。

### DB 製品 (Q1)

- PostgreSQL 18.x (採用 image tag `postgres:18.3`、patch まで pin)
- Patch 更新ポリシー: minor / patch update を取り込む際は CHANGELOG を確認し、互換性に問題がなければ image tag を直接書き換える (PR で承認)。本番運用は別リポジトリで並走するため、本リポジトリの値はリファレンス参照点

### クライアントライブラリ (Q3)

- `github.com/jackc/pgx/v5` (`pgxpool.Pool`) を直接使用
- `database/sql` ラッパは原則使わない (機能重複)
- `sqlc` は M2.x で本格導入 (Repository 層実装と合わせて)。本マイルストーンの範囲外

### マイグレーションツール (Q2)

- `github.com/golang-migrate/migrate/v4` を CLI binary として運用
- バージョン pin: `v4.19.1` (`core/Makefile` の `MIGRATE_VERSION` 変数で固定)
- 起動時の整合性検証 (`AssertClean`) のみ Go library API として `core/internal/dbmigrate` から呼び出す。実 migrate の up/down は CLI 経由 (`make migrate-up` 等) のみ (Q9 起動と migrate 分離方針)

### 環境変数 11 個 (Q7 / Q10 / Q11)

接続 (6 個):

| Key                | 既定値    | 範囲 / 備考                                                                                        |
| ------------------ | --------- | -------------------------------------------------------------------------------------------------- |
| `CORE_DB_HOST`     | (必須)    | 任意のホスト名 / IP                                                                                |
| `CORE_DB_PORT`     | (必須)    | 1〜65535 の整数                                                                                    |
| `CORE_DB_USER`     | (必須)    | PostgreSQL ユーザー                                                                                |
| `CORE_DB_PASSWORD` | (必須)    | パスワード (独立 env、F-1 最低ライン)                                                              |
| `CORE_DB_NAME`     | (必須)    | DB 名                                                                                              |
| `CORE_DB_SSLMODE`  | `disable` | 6 種: `disable` / `allow` / `prefer` / `require` / `verify-ca` / `verify-full`、それ以外は起動失敗 |

プール (5 個、`pgxpool.Config` に反映):

| Key                                | 既定値 | 範囲 / 備考                         |
| ---------------------------------- | ------ | ----------------------------------- |
| `CORE_DB_POOL_MAX_CONNS`           | `10`   | 正数のみ                            |
| `CORE_DB_POOL_MIN_CONNS`           | `1`    | 0 以上、`MAX_CONNS` 以下            |
| `CORE_DB_POOL_MAX_CONN_LIFETIME`   | `5m`   | `time.ParseDuration` 互換、負数禁止 |
| `CORE_DB_POOL_MAX_CONN_IDLE_TIME`  | `2m`   | 同上                                |
| `CORE_DB_POOL_HEALTH_CHECK_PERIOD` | `30s`  | 同上                                |

### マイグレーションファイル命名規則 (Q4)

- `core/db/migrations/` 配下に配置
- ファイル名: `{8 桁連番}_{snake_case slug}.{up|down}.sql`
- 例: `00000001_smoke_initial.up.sql` / `00000001_smoke_initial.down.sql`
- 連番衝突は `make lint` の `migration sequence collision check` で自動検出 (`.up.sql` のみ集計してペアの up/down で誤検知しない)

### トランザクション境界 (Q5)

- 1 ファイル = 1 TX (`golang-migrate` v4 が暗黙的に `BEGIN/COMMIT` で wrap する仕様に依存)
- SQL 内に明示的な `BEGIN` / `COMMIT` を書いてはならない (二重 TX エラー)
- `CREATE INDEX CONCURRENTLY` 等の TX 内不可 DDL は別ファイル分離 + 当面回避 (本マイルストーンでは未対応、必要時に migrate の `-x` オプションを検討)

### SSL/TLS 接続要件 (Q10)

- 本番想定で `disable` / `allow` / `prefer` は不可。**必ず** `require` / `verify-ca` / `verify-full` のいずれか
- 本番推奨は `verify-full` (RDS / CloudSQL は提供元 root CA を `sslrootcert` env で指定)
- 開発環境 (docker compose) は `disable` で許可。本リポジトリの `docker/.env.sample` も既定 `disable`

### マイグレーション運用 (Q9)

- 起動シーケンスから migrate up を**呼ばない**: `make migrate-up` を CLI で実行するフローと分離
- 起動時は `dbmigrate.AssertClean` のみ呼び、`schema_migrations.dirty=true` を検出したら `os.Exit(1)` (F-13 start gate)
- 起動シーケンス順序 (Q9):

  ```
  logger.Default()
  → config.Load()
  → ctx (event_id 付与)
  → db.Open(ctx, cfg.Database, l)   // 内部で pgxpool 構築 + 初回 Ping
  → defer pool.Close()
  → dbmigrate.AssertClean(ctx, dsn, file://db/migrations, l)   // dirty なら exit 1
  → server.New(cfg, l, pool)
  → ListenAndServe
  ```

- migrate のパス (`migrationsSource`) は `CORE_MIGRATIONS_DIR` env で override 可能 (絶対パス推奨)。未設定時は `file://db/migrations` (`make run` 経由の cwd=`core/` 前提)

### 開発者向け運用手順

1. **初回セットアップ**: `cp docker/.env.sample docker/.env` → `docker compose -f docker/compose.yaml up -d postgres` → `make -C core migrate-install`
2. **マイグレーション適用**: 開発 DB へは `make -C core migrate-up`、テスト DB へは `make -C core test-integration` (内部で drop + up)
3. **新規マイグレーション**: `make -C core migrate-create NAME=<slug>` で雛形生成 → `up.sql` / `down.sql` 双方を手書き
4. **dirty 復旧**: 起動時に `dbmigrate: schema_migrations is dirty` エラーが出たら、`make -C core migrate-force VERSION=<n>` で強制リセット → 修正後 `make migrate-up` 再実行
5. **テスト用 DB**: `id_core_test` を別建て (compose 同居で OK)、`TEST_DATABASE_URL` env で override 可能

## API (OIDC OP / 標準エンドポイント)

TBD — `/authorize`, `/token`, `/userinfo`, `/jwks.json`, `/.well-known/openid-configuration` の規約 (M1.x で確定)

### 既存エンドポイント

| Path      | Method | 認証 | 概要                                 |
| --------- | ------ | ---- | ------------------------------------ |
| `/health` | GET    | 不要 | サーバー稼働確認 (`{"status":"ok"}`) |

## API (id-core 独自管理 API)

TBD — 内部ユーザー管理・アカウントリンク・電話番号認証・SNS 認証 (LINE 等) のエンドポイント規約

## エラーハンドリング

エラー型と JSON シリアライズは `core/internal/apperror/` パッケージで一元管理する (M0.2 で導入)。

### 基本形 (F-7)

内部 API のエラーレスポンスは下記の JSON 形式で返却する:

```json
{
  "code": "INVALID_PARAMETER",
  "message": "ポートは 1〜65535 の整数で指定してください",
  "details": { "field": "CORE_PORT", "received": "0" },
  "request_id": "01890000-0000-7000-8000-000000000000"
}
```

| フィールド   | 型     | 必須 | 内容                                                                                                               |
| ------------ | ------ | ---- | ------------------------------------------------------------------------------------------------------------------ |
| `code`       | string | 必須 | `SCREAMING_SNAKE_CASE` のエラーコード (例: `INVALID_PARAMETER` / `INTERNAL_ERROR`)                                 |
| `message`    | string | 必須 | 人間可読の本文 (M0.2〜M5.x スコープでは日本語、OIDC エンドポイント側は RFC 6749 準拠の英語フレーズ等を別途検討)    |
| `details`    | object | 任意 | 補足情報 (object のみ。配列が必要なら object のキー配下にネスト)。シークレットを含めない (redact deny-list と整合) |
| `request_id` | string | 必須 | リクエストの request_id (HTTP 経路) または起動 / ジョブの event_id (非 HTTP 経路)                                  |

### エラーコード命名規則

- `SCREAMING_SNAKE_CASE` で表記する
- ドメイン語彙とエラー種別を組み合わせる (例: `ACCOUNT_NOT_FOUND` / `OTP_EXPIRED`)
- panic 等の予期しないエラーには固定値 `INTERNAL_ERROR` を返す (F-9 / F-10)
- OIDC エンドポイント (M1.x 以降) は RFC 6749 / 6750 / OpenID Connect Core が定める標準コード (`invalid_request` / `invalid_grant` / `invalid_token` 等) を併用する。本規約の `SCREAMING_SNAKE_CASE` は内部 API スコープに適用し、OIDC 標準エンドポイントは仕様準拠を優先する

### details の制約

- 型は object に限定 (`apperror.WithDetails(map[string]any)`)。配列を含めたい場合は object のキー配下にネストする
- シークレットを含めない。redact deny-list (後述) と同一の命名規約に従い、deny-list 該当キーを `details` に詰めない
- `details` を含めたエラーは内部 API のクライアント側 (フロントエンド) でフィールド単位の誘導 UI に活用する

### redact 対象キー一覧 (Q8 完全一覧)

ログ・エラーレスポンス出力前に値を `[REDACTED]` (固定値) に置換する。照合は **case-insensitive かつ完全一致**、ネスト object / 配列を再帰走査する。

- ヘッダ (6 件): `Authorization`, `Cookie`, `Set-Cookie`, `Proxy-Authorization`, `X-Api-Key`, `X-Auth-Token`
- フィールド (16 件): `password`, `current_password`, `new_password`, `access_token`, `refresh_token`, `id_token`, `code`, `code_verifier`, `client_secret`, `assertion`, `client_assertion`, `private_key`, `secret`, `api_key`, `jwt`, `bearer_token`

実装は `core/internal/logger/redact.go` に集約。クエリ文字列・form パラメータ・ログ attribute・`details` の出力経路で同一の deny-list を再利用する (二重管理禁止)。部分一致は誤検知防止のため禁止。

### panic 時の挙動 (F-9 / F-10)

- middleware の `recover` が panic を捕捉 → HTTP 500 + `{ "code": "INTERNAL_ERROR", "message": "...", "request_id": "..." }` を返却
- スタックトレースは内部ログ (ERROR レベル) にのみ記録し、レスポンス本文には含めない (情報漏えい対策)
- スタック取得には `runtime.Stack` を使用し、長さ上限を設ける

## ロギング・テレメトリ

`core/internal/logger/` で構造化ログを一元提供する (M0.2 で導入)。

### 実装スタック

- ロガー: Go 標準 `log/slog` を薄くラップ (`core/internal/logger/Logger`)
- フォーマット切替: `CORE_LOG_FORMAT=json|text` 環境変数 (既定 `json`、開発時は `text`、不正値は起動失敗)
- 一意 ID 生成: UUID v7 (`github.com/google/uuid` v1.6+ の `uuid.NewV7`)。`uuid.New` / `uuid.NewRandom` (v4) は禁止
- 出力先: 本番は標準出力 (1 行 1 レコードの JSON Lines)、テストは `bytes.Buffer`

### `time` フィールド

- 値は **RFC3339Nano UTC** (例: `2026-05-02T01:23:45.678901234Z`、`Z` suffix 強制)
- `slog.HandlerOptions.ReplaceAttr` フックで UTC + `RFC3339Nano` に変換する
- `time.Local = time.UTC` のようなプロセスグローバルな副作用は禁止 (他パッケージ・他テストへの汚染回避)

### ログレベル使い分け (Q7)

| レベル  | 用途                                                                      |
| ------- | ------------------------------------------------------------------------- |
| `DEBUG` | 本番では出力しない (フィルタ運用)。開発調査時のみ                         |
| `INFO`  | 業務イベント・正常系の処理経過 (ログイン成功、リソース作成、起動・終了等) |
| `WARN`  | クライアント起因の異常 (4xx)、復旧可能な障害、構成警告                    |
| `ERROR` | サーバ起因の異常 (5xx)、panic、永続化失敗等の運用調査が必要な事象         |

### 必須フィールド

#### HTTP 経路 (access_log / handler ログ)

| フィールド    | 型     | 必須 | 内容                                                                      |
| ------------- | ------ | ---- | ------------------------------------------------------------------------- |
| `time`        | string | 必須 | RFC3339Nano UTC                                                           |
| `level`       | string | 必須 | `DEBUG` / `INFO` / `WARN` / `ERROR`                                       |
| `msg`         | string | 必須 | ログメッセージ (`access` 等の固定 kind を別 attribute で明示する場合あり) |
| `request_id`  | string | 必須 | UUID v7 (X-Request-Id 由来 or サーバ生成)                                 |
| `method`      | string | 必須 | HTTP メソッド                                                             |
| `path`        | string | 必須 | URL パス + (redact 済み) クエリ文字列                                     |
| `status`      | number | 必須 | HTTP ステータスコード                                                     |
| `duration_ms` | number | 必須 | 処理時間 (ミリ秒、float)                                                  |

#### 非 HTTP 経路 (起動 / signal handler / ジョブ等)

| フィールド | 型     | 必須 | 内容                                |
| ---------- | ------ | ---- | ----------------------------------- |
| `time`     | string | 必須 | RFC3339Nano UTC                     |
| `level`    | string | 必須 | `DEBUG` / `INFO` / `WARN` / `ERROR` |
| `msg`      | string | 必須 | ログメッセージ                      |
| `event_id` | string | 必須 | UUID v7 (起動毎・ジョブ毎の一意 ID) |

追加フィールドは許容 (前方互換)。フィールド削除・型変更は **`core/internal/logger/contract_test.go` の F-16 契約テストで失敗** する破壊的変更扱い。

### `request_id` / `event_id` の伝播 (F-3 / F-4)

- `logger.WithRequestID(ctx, id)` / `logger.RequestIDFrom(ctx)` で HTTP 経路に付与
- `logger.WithEventID(ctx, id)` / `logger.EventIDFrom(ctx)` で非 HTTP 経路に付与
- HTTP 経路の必須付与は `request_id` middleware (D1 順序の最外層) が担保する
- 非 HTTP 経路の必須付与は `cmd/<binary>/main.go` 起動時とジョブランナー側で担保する
- Domain 層 (将来導入) は context から取り出すのみ、ロガー直呼び出しは禁止 (F-14)

### ログ出力失敗時の挙動 (Q9)

- primary writer (stdout) 失敗時に **stderr にフォールバック**
- stderr も失敗した場合は **atomic drop counter** を増分してリクエスト処理を継続 (リクエストを止めない)
- `core/internal/logger/FallbackWriter.DropCount()` で counter を取得 (M1.x のメトリクス連携で公開予定)

### ログメッセージ言語

- 内部 API スコープでは `msg` / `error` フィールドを **日本語**で記述する
- ただしフィールド名 (`request_id` / `status` 等) は英語の予約語を使用する
- OIDC 標準エンドポイント (M1.x 以降) は OAuth 2.0 / OIDC 仕様の `error` / `error_description` 規約に従う

### ログインジェクション対策 (F-1)

- ロガーは `slog` の構造化 attribute (key/value) のみを公開し、文字列連結による出力は許容しない
- `Error(ctx, msg, err, args...)` は `err.Error()` を構造化 attribute として渡し、改行・制御文字を JSON エンコーダ経由で安全にエスケープする

## middleware 構成

HTTP middleware の実行順序 (D1) を以下に統一する:

```
request_id  →  access_log  →  recover  →  handler
```

| middleware   | 責務                                                                                                                                                                         |
| ------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `request_id` | クライアント `X-Request-Id` の検証 + 不正なら UUID v7 を新規発番。サニタイズ済みの不正値は `client_request_id` として context に残す。レスポンスヘッダ `X-Request-Id` を設定 |
| `access_log` | 終了時に 1 行のアクセスログを構造化出力 (5xx=ERROR / 4xx=WARN / それ以外=INFO)。`recover` が変換した最終 status を観測する                                                   |
| `recover`    | panic を捕捉して 5xx + `INTERNAL_ERROR` を返す。スタックトレースを ERROR レベルでログ出力                                                                                    |

D1 順序の根拠: 全ログレコード (panic 含む) に `request_id` を付与するため、`request_id` を最外層に置く。`access_log` を `recover` より外側に置くことで、`recover` 後の最終 status (5xx) を access ログに反映する。

## 認可

TBD — id-core 自身の管理 API の認可方式 (CLAUDE.md 方針: IAM ミドルウェア不採用、手書きポリシー)
