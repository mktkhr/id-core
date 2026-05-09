# バックエンドレジストリ (id-core / Go)

> 最終更新: 2026-05-09 (M1.1: keystore / OIDC discovery / jwks / notimpl / devkeygen / `CORE_ENV` / `CORE_OIDC_*` を追加、設計 #32)

## パッケージマッピング

| パス                            | 用途                                                                                                                    | 依存                                                                                                                                                                                                 |
| ------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `core/cmd/core`                 | 実行ファイルエントリポイント (`main`)。起動・signal handling・最終 ID 発番・logger 初期化                               | `internal/config`, `internal/logger`, `internal/server`, `internal/keystore`, `os`, `os/signal`, `context`                                                                                           |
| `core/cmd/devkeygen`            | dev / staging 用 RSA 2048 bit 鍵ペア生成 CLI (M1.1、`make dev-keygen`)                                                  | `crypto/rsa`, `crypto/x509`, `encoding/pem`, `flag`, `os`                                                                                                                                            |
| `core/internal/config`          | 環境変数読み込み + バリデーション (`CORE_ENV` strict + `CORE_OIDC_*` 追加、M1.1)                                        | `os`, `strconv`, `strings`, `net/url`, `fmt`, `time`                                                                                                                                                 |
| `core/internal/logger`          | 構造化ロガー (slog ラッパ + Format / Context / Redact / FallbackWriter)                                                 | `log/slog`, `github.com/google/uuid`, `context`, `io`, `sync/atomic`                                                                                                                                 |
| `core/internal/apperror`        | 構造化エラー型 (`CodedError`) + JSON シリアライザ (`Response` / `WriteJSON`)                                            | `encoding/json`, `errors`, `fmt`, `net/http`                                                                                                                                                         |
| `core/internal/middleware`      | request_id / access_log / recover (D1 順序の HTTP middleware 群)                                                        | `internal/logger`, `internal/apperror`, `github.com/google/uuid`, `net/http`, `sync/atomic`                                                                                                          |
| `core/internal/server`          | `*http.Server` 構築 + ハンドラ登録 + middleware チェーン組み込み (M1.1 で `ks` 引数追加 + OIDC route 統合)              | `internal/config`, `internal/health`, `internal/keystore`, `internal/logger`, `internal/middleware`, `internal/oidc/discovery`, `internal/oidc/jwks`, `internal/oidc/notimpl`, `pgxpool`, `net/http` |
| `core/internal/health`          | `/health` (M0.1) / `/health/live` / `/health/ready` (M0.3) ハンドラ                                                     | `encoding/json`, `internal/logger`, `pgxpool`, `net/http`                                                                                                                                            |
| `core/internal/db`              | PostgreSQL 接続層 (`Open` / `BuildDSN` / `SafeRepr`)。`pgxpool.Pool` を生成                                             | `internal/config`, `internal/logger`, `github.com/jackc/pgx/v5`, `pgxpool`, `net/url`                                                                                                                |
| `core/internal/dbmigrate`       | マイグレーション library API (`RunUp` / `RunDown` / `AssertClean` / `ErrDirty`)                                         | `internal/logger`, `github.com/golang-migrate/migrate/v4`, `migrate/database/postgres`, `migrate/source/file`                                                                                        |
| `core/internal/testutil/dbtest` | 統合テスト用 DB ヘルパー (`NewPool` / `BeginTx` / `RollbackTx`)                                                         | `github.com/jackc/pgx/v5`, `pgxpool`, `testing`                                                                                                                                                      |
| `core/internal/keystore`        | OIDC 署名鍵 (RSA 公開/秘密鍵) の保管 + 読み込み + kid 算出 (M1.1)。`KeySet` I/F + `staticKeySet` + `Init` + `DeriveKid` | `crypto/rsa`, `crypto/sha256`, `crypto/x509`, `encoding/hex`, `encoding/pem`, `internal/logger`                                                                                                      |
| `core/internal/oidc/discovery`  | `GET /.well-known/openid-configuration` ハンドラ + メタデータ構築 + 決定的シリアライズ + ETag (M1.1)                    | `crypto/sha256`, `encoding/base64`, `encoding/json`, `internal/config`, `net/http`                                                                                                                   |
| `core/internal/oidc/jwks`       | `GET /jwks` ハンドラ + jwx/v3 経由で公開鍵 → JWK 変換 + 決定的シリアライズ + ETag (M1.1)                                | `github.com/lestrrat-go/jwx/v3`, `crypto/sha256`, `encoding/base64`, `encoding/json`, `internal/keystore`                                                                                            |
| `core/internal/oidc/notimpl`    | M1.2-1.4 で本実装される endpoint の 503 stub (M1.1、F-23、`Handler(milestone string)` ファクトリ)                       | `encoding/json`, `net/http`                                                                                                                                                                          |

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

| Path                                | Method | 認証 | 担当パッケージ            | 状態                     | 追加マイルストーン |
| ----------------------------------- | ------ | ---- | ------------------------- | ------------------------ | ------------------ |
| `/.well-known/openid-configuration` | GET    | 不要 | `internal/oidc/discovery` | 本実装 (Discovery 1.0)   | M1.1               |
| `/jwks`                             | GET    | 不要 | `internal/oidc/jwks`      | 本実装 (RFC 7517 準拠)   | M1.1               |
| `/authorize`                        | GET    | 不要 | `internal/oidc/notimpl`   | 503 stub (本実装は M1.2) | M1.1               |
| `/token`                            | POST   | 不要 | `internal/oidc/notimpl`   | 503 stub (本実装は M1.3) | M1.1               |
| `/userinfo`                         | GET    | 不要 | `internal/oidc/notimpl`   | 503 stub (本実装は M1.4) | M1.1               |

### id-core 管理 API

TBD

### 共通

| Path            | Method | 認証 | 担当パッケージ    | 追加マイルストーン |
| --------------- | ------ | ---- | ----------------- | ------------------ |
| `/health`       | GET    | 不要 | `internal/health` | M0.1               |
| `/health/live`  | GET    | 不要 | `internal/health` | M0.3               |
| `/health/ready` | GET    | 不要 | `internal/health` | M0.3               |

## 環境変数一覧

| Key                                | 既定値    | 範囲                                                                         | 必須   | 追加マイルストーン | 説明                                                                                          |
| ---------------------------------- | --------- | ---------------------------------------------------------------------------- | ------ | ------------------ | --------------------------------------------------------------------------------------------- |
| `CORE_PORT`                        | `8080`    | 1〜65535 の整数                                                              | 任意   | M0.1               | core HTTP サーバーのリッスンポート                                                            |
| `CORE_LOG_FORMAT`                  | `json`    | `json` または `text`                                                         | 任意   | M0.2               | ログ出力フォーマット (本番=`json` / 開発=`text`)。不正値で起動失敗                            |
| `CORE_DB_HOST`                     | (必須)    | 任意のホスト / IP                                                            | 必須   | M0.3               | PostgreSQL 接続先ホスト                                                                       |
| `CORE_DB_PORT`                     | (必須)    | 1〜65535                                                                     | 必須   | M0.3               | PostgreSQL 接続ポート                                                                         |
| `CORE_DB_USER`                     | (必須)    | -                                                                            | 必須   | M0.3               | DB ユーザー                                                                                   |
| `CORE_DB_PASSWORD`                 | (必須)    | -                                                                            | 必須   | M0.3               | DB パスワード (独立 env、F-1)                                                                 |
| `CORE_DB_NAME`                     | (必須)    | -                                                                            | 必須   | M0.3               | DB 名                                                                                         |
| `CORE_DB_SSLMODE`                  | `disable` | `disable` / `allow` / `prefer` / `require` / `verify-ca` / `verify-full`     | 任意   | M0.3               | SSL/TLS モード (Q10)                                                                          |
| `CORE_DB_POOL_MAX_CONNS`           | `10`      | 1 以上                                                                       | 任意   | M0.3               | pgxpool 最大接続数                                                                            |
| `CORE_DB_POOL_MIN_CONNS`           | `1`       | 0 以上、`MAX_CONNS` 以下                                                     | 任意   | M0.3               | pgxpool 最小接続数 (cold start 対策)                                                          |
| `CORE_DB_POOL_MAX_CONN_LIFETIME`   | `5m`      | `time.ParseDuration` 互換、負数禁止                                          | 任意   | M0.3               | コネクション最大生存時間                                                                      |
| `CORE_DB_POOL_MAX_CONN_IDLE_TIME`  | `2m`      | 同上                                                                         | 任意   | M0.3               | コネクション最大アイドル時間                                                                  |
| `CORE_DB_POOL_HEALTH_CHECK_PERIOD` | `30s`     | 同上                                                                         | 任意   | M0.3               | プールヘルスチェック周期                                                                      |
| `CORE_MIGRATIONS_DIR`              | (任意)    | 絶対パス推奨 (`file://...` 前置可)                                           | 任意   | M0.3               | `dbmigrate.AssertClean` が参照する migrations ディレクトリ。未設定なら `file://db/migrations` |
| `TEST_DATABASE_URL`                | (任意)    | PostgreSQL DSN                                                               | 任意   | M0.3               | 統合テスト用 DSN。未設定なら fallback (`localhost:5432/id_core_test`)                         |
| `TEST_DB_REQUIRED`                 | (任意)    | `1` で必須化、それ以外で skip 許容                                           | 任意   | M0.3               | CI で `1` を設定し、DB 接続失敗を skip ではなく fail に                                       |
| `CORE_ENV`                         | (必須)    | `prod` / `staging` / `dev` (strict 3 値、それ以外で起動失敗)                 | 必須   | M1.1               | 環境識別子。鍵ソース必須性 + issuer scheme の判定に使う (Q7、設計 #32)                        |
| `CORE_OIDC_ISSUER`                 | (必須)    | `https://...` (`prod`/`staging`)、`http://`/`https://` (`dev`)、末尾 / strip | 必須   | M1.1               | OP の論理識別子 URL (F-1)                                                                     |
| `CORE_OIDC_KEY_FILE`               | (条件付)  | PEM PKCS#8 秘密鍵ファイルパス                                                | 条件付 | M1.1               | `prod` で必須、`staging`/`dev` は `CORE_OIDC_DEV_GENERATE_KEY=1` で代替可 (F-7 / F-9)         |
| `CORE_OIDC_DEV_GENERATE_KEY`       | (任意)    | `0` または `1`                                                               | 任意   | M1.1               | `1` で起動時 RSA 2048 bit 鍵生成 (メモリ保持)。`prod` では強制無効 (起動失敗)                 |
| `CORE_OIDC_KEY_ID`                 | (任意)    | 任意の文字列                                                                 | 任意   | M1.1               | kid 固定値。未設定時は keystore で公開鍵 DER SHA-256 先頭 24 hex を自動算出 (F-11)            |
| `CORE_OIDC_JWKS_MAX_AGE`           | `300`     | 0〜86400 秒                                                                  | 任意   | M1.1               | JWKS Cache-Control max-age (F-6)                                                              |
| `CORE_OIDC_DISCOVERY_MAX_AGE`      | `0`       | 0〜86400 秒                                                                  | 任意   | M1.1               | Discovery Cache-Control max-age (0 → no-cache, must-revalidate / >0 → public, max-age=N、F-1) |
| `CORE_OIDC_AUTHORIZATION_ENDPOINT` | (任意)    | 絶対 URL                                                                     | 任意   | M1.1               | 未設定なら issuer + `/authorize` (F-3 / F-17、論点 #12)                                       |
| `CORE_OIDC_TOKEN_ENDPOINT`         | (任意)    | 絶対 URL                                                                     | 任意   | M1.1               | 未設定なら issuer + `/token`                                                                  |
| `CORE_OIDC_USERINFO_ENDPOINT`      | (任意)    | 絶対 URL                                                                     | 任意   | M1.1               | 未設定なら issuer + `/userinfo`                                                               |
| `CORE_OIDC_JWKS_URI`               | (任意)    | 絶対 URL                                                                     | 任意   | M1.1               | 未設定なら issuer + `/jwks` (Q9: 拡張子なし)                                                  |

## エラーコード一覧

### 共通 (内部 API)

| code                       | HTTP status | 用途                                                                                                                                                                         | 追加マイルストーン |
| -------------------------- | ----------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------ |
| `INTERNAL_ERROR`           | 500         | panic 等の予期しないエラー (`recover` middleware が固定で返す / F-9 / F-10)                                                                                                  | M0.2               |
| `ENDPOINT_NOT_IMPLEMENTED` | 503         | M1.2-1.4 で本実装される endpoint の予約 (M1.1 で定数導入、現行 notimpl handler は OIDC 標準形式 `endpoint_not_implemented` snake_case 直接出力で本 code 経由しない、論点 #8) | M1.1               |

### OIDC 標準エンドポイント

OIDC 標準エンドポイント (M1.x 以降) は RFC 6749 / 6750 / OpenID Connect Core が定める標準コード (`invalid_request` / `invalid_grant` / `invalid_token` / `invalid_client` 等) を採用する。本レジストリの `SCREAMING_SNAKE_CASE` は内部 API スコープに適用し、OIDC 標準コードは仕様準拠を優先する。

詳細な命名規則 / `details` 制約 / panic 時の挙動は `docs/context/backend/conventions.md` の「エラーハンドリング」節を参照。

## マイグレーション一覧

| 連番       | ファイル名                             | 用途                                                                       | 追加マイルストーン |
| ---------- | -------------------------------------- | -------------------------------------------------------------------------- | ------------------ |
| `00000001` | `00000001_smoke_initial.{up,down}.sql` | smoke テーブル `schema_smoke` を作成 / 削除 (F-14 double-roundtrip 検証用) | M0.3               |
