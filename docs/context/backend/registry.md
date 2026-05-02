# バックエンドレジストリ (id-core / Go)

> 最終更新: 2026-05-02 (M0.3: DB 接続 / マイグレーション / 統合テスト基盤を追加)

## パッケージマッピング

| パス                            | 用途                                                                                       | 依存                                                                                                          |
| ------------------------------- | ------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------- |
| `core/cmd/core`                 | 実行ファイルエントリポイント (`main`)。起動・signal handling・最終 ID 発番・logger 初期化  | `internal/config`, `internal/logger`, `internal/server`, `os`, `os/signal`, `context`                         |
| `core/internal/config`          | 環境変数読み込み + バリデーション                                                          | `os`, `strconv`, `fmt`                                                                                        |
| `core/internal/logger`          | 構造化ロガー (slog ラッパ + Format / Context / Redact / FallbackWriter)                    | `log/slog`, `github.com/google/uuid`, `context`, `io`, `sync/atomic`                                          |
| `core/internal/apperror`        | 構造化エラー型 (`CodedError`) + JSON シリアライザ (`Response` / `WriteJSON`)               | `encoding/json`, `errors`, `fmt`, `net/http`                                                                  |
| `core/internal/middleware`      | request_id / access_log / recover (D1 順序の HTTP middleware 群)                           | `internal/logger`, `internal/apperror`, `github.com/google/uuid`, `net/http`, `sync/atomic`                   |
| `core/internal/server`          | `*http.Server` 構築 + ハンドラ登録 + middleware チェーン組み込み (M0.3 で `pool` 引数追加) | `internal/config`, `internal/health`, `internal/logger`, `internal/middleware`, `pgxpool`, `net/http`         |
| `core/internal/health`          | `/health` (M0.1) / `/health/live` / `/health/ready` (M0.3) ハンドラ                        | `encoding/json`, `internal/logger`, `pgxpool`, `net/http`                                                     |
| `core/internal/db`              | PostgreSQL 接続層 (`Open` / `BuildDSN` / `SafeRepr`)。`pgxpool.Pool` を生成                | `internal/config`, `internal/logger`, `github.com/jackc/pgx/v5`, `pgxpool`, `net/url`                         |
| `core/internal/dbmigrate`       | マイグレーション library API (`RunUp` / `RunDown` / `AssertClean` / `ErrDirty`)            | `internal/logger`, `github.com/golang-migrate/migrate/v4`, `migrate/database/postgres`, `migrate/source/file` |
| `core/internal/testutil/dbtest` | 統合テスト用 DB ヘルパー (`NewPool` / `BeginTx` / `RollbackTx`)                            | `github.com/jackc/pgx/v5`, `pgxpool`, `testing`                                                               |

## Feature ディレクトリマッピング

M0.2 では feature パッケージなし (機能横断的な骨格 + ログ・エラー基盤のみ)。
M1.x の OIDC 実装で `core/internal/features/<feature>/` が登場する。

## テーブル一覧

M0.3 では実テーブルなし。M0.3 範囲はマイグレーション基盤の検証のみで、smoke テーブル `schema_smoke` (id / label / note) のみ存在。実テーブル DDL は M2.x 以降。

### マスターテーブル

TBD (M2.x で本格化)

### エンティティテーブル

TBD (M2.x で本格化)

### 中間テーブル

TBD (M2.x で本格化)

### 検証用テーブル (M0.3 のみ)

| テーブル名     | 用途                                                                               | 追加マイルストーン |
| -------------- | ---------------------------------------------------------------------------------- | ------------------ |
| `schema_smoke` | マイグレーション基盤の double-roundtrip 検証用 (本番では使わない、M2.x で削除候補) | M0.3               |

## API エンドポイント一覧

### OIDC 標準エンドポイント

TBD — `/authorize`, `/token`, `/userinfo`, `/jwks.json`, `/.well-known/openid-configuration` 等 (M1.x で確定)

### id-core 管理 API

TBD

### 共通

| Path            | Method | 認証 | 担当パッケージ    | 追加マイルストーン |
| --------------- | ------ | ---- | ----------------- | ------------------ |
| `/health`       | GET    | 不要 | `internal/health` | M0.1               |
| `/health/live`  | GET    | 不要 | `internal/health` | M0.3               |
| `/health/ready` | GET    | 不要 | `internal/health` | M0.3               |

## 環境変数一覧

| Key                                | 既定値    | 範囲                                                                     | 必須 | 追加マイルストーン | 説明                                                                                          |
| ---------------------------------- | --------- | ------------------------------------------------------------------------ | ---- | ------------------ | --------------------------------------------------------------------------------------------- |
| `CORE_PORT`                        | `8080`    | 1〜65535 の整数                                                          | 任意 | M0.1               | core HTTP サーバーのリッスンポート                                                            |
| `CORE_LOG_FORMAT`                  | `json`    | `json` または `text`                                                     | 任意 | M0.2               | ログ出力フォーマット (本番=`json` / 開発=`text`)。不正値で起動失敗                            |
| `CORE_DB_HOST`                     | (必須)    | 任意のホスト / IP                                                        | 必須 | M0.3               | PostgreSQL 接続先ホスト                                                                       |
| `CORE_DB_PORT`                     | (必須)    | 1〜65535                                                                 | 必須 | M0.3               | PostgreSQL 接続ポート                                                                         |
| `CORE_DB_USER`                     | (必須)    | -                                                                        | 必須 | M0.3               | DB ユーザー                                                                                   |
| `CORE_DB_PASSWORD`                 | (必須)    | -                                                                        | 必須 | M0.3               | DB パスワード (独立 env、F-1)                                                                 |
| `CORE_DB_NAME`                     | (必須)    | -                                                                        | 必須 | M0.3               | DB 名                                                                                         |
| `CORE_DB_SSLMODE`                  | `disable` | `disable` / `allow` / `prefer` / `require` / `verify-ca` / `verify-full` | 任意 | M0.3               | SSL/TLS モード (Q10)                                                                          |
| `CORE_DB_POOL_MAX_CONNS`           | `10`      | 1 以上                                                                   | 任意 | M0.3               | pgxpool 最大接続数                                                                            |
| `CORE_DB_POOL_MIN_CONNS`           | `1`       | 0 以上、`MAX_CONNS` 以下                                                 | 任意 | M0.3               | pgxpool 最小接続数 (cold start 対策)                                                          |
| `CORE_DB_POOL_MAX_CONN_LIFETIME`   | `5m`      | `time.ParseDuration` 互換、負数禁止                                      | 任意 | M0.3               | コネクション最大生存時間                                                                      |
| `CORE_DB_POOL_MAX_CONN_IDLE_TIME`  | `2m`      | 同上                                                                     | 任意 | M0.3               | コネクション最大アイドル時間                                                                  |
| `CORE_DB_POOL_HEALTH_CHECK_PERIOD` | `30s`     | 同上                                                                     | 任意 | M0.3               | プールヘルスチェック周期                                                                      |
| `CORE_MIGRATIONS_DIR`              | (任意)    | 絶対パス推奨 (`file://...` 前置可)                                       | 任意 | M0.3               | `dbmigrate.AssertClean` が参照する migrations ディレクトリ。未設定なら `file://db/migrations` |
| `TEST_DATABASE_URL`                | (任意)    | PostgreSQL DSN                                                           | 任意 | M0.3               | 統合テスト用 DSN。未設定なら fallback (`localhost:5432/id_core_test`)                         |
| `TEST_DB_REQUIRED`                 | (任意)    | `1` で必須化、それ以外で skip 許容                                       | 任意 | M0.3               | CI で `1` を設定し、DB 接続失敗を skip ではなく fail に                                       |

## エラーコード一覧

### 共通 (内部 API)

| code             | HTTP status | 用途                                                                        | 追加マイルストーン |
| ---------------- | ----------- | --------------------------------------------------------------------------- | ------------------ |
| `INTERNAL_ERROR` | 500         | panic 等の予期しないエラー (`recover` middleware が固定で返す / F-9 / F-10) | M0.2               |

### OIDC 標準エンドポイント

OIDC 標準エンドポイント (M1.x 以降) は RFC 6749 / 6750 / OpenID Connect Core が定める標準コード (`invalid_request` / `invalid_grant` / `invalid_token` / `invalid_client` 等) を採用する。本レジストリの `SCREAMING_SNAKE_CASE` は内部 API スコープに適用し、OIDC 標準コードは仕様準拠を優先する。

詳細な命名規則 / `details` 制約 / panic 時の挙動は `docs/context/backend/conventions.md` の「エラーハンドリング」節を参照。

## マイグレーション一覧

| 連番       | ファイル名                             | 用途                                                                       | 追加マイルストーン |
| ---------- | -------------------------------------- | -------------------------------------------------------------------------- | ------------------ |
| `00000001` | `00000001_smoke_initial.{up,down}.sql` | smoke テーブル `schema_smoke` を作成 / 削除 (F-14 double-roundtrip 検証用) | M0.3               |
