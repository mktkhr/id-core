# core/ — id-core OIDC OP 本体

id-core の OIDC OP (Identity Provider) 本体の Go 実装。

## 前提

- **Go**: `1.22+` (動作確認: `1.26.2`)
- POSIX 準拠シェル + `make`
- **PostgreSQL 18.x** (M0.3 以降。`docker compose -f docker/compose.yaml up -d postgres` で起動)
- **golang-migrate CLI v4.19.1** (`make migrate-install` でインストール)

## ディレクトリ構成 (M1.1 時点)

```
core/
├── cmd/
│   ├── core/main.go           # エントリポイント (起動・signal handling・db.Open・AssertClean・keystore.Init)
│   └── devkeygen/main.go      # dev / staging 用 RSA 2048 bit 鍵生成 CLI (M1.1、make dev-keygen)
├── db/migrations/             # マイグレーション SQL (8 桁連番命名)
├── dev-keys/                  # devkeygen 出力先 (.gitignore 除外、リポジトリにコミットしない)
├── internal/
│   ├── config/                # 環境変数読み込み + バリデーション (CORE_*, CORE_DB_*, CORE_ENV, CORE_OIDC_*)
│   ├── logger/                # 構造化ロガー (log/slog ラッパ + redact + fallback)
│   ├── apperror/              # 構造化エラー型 + JSON シリアライザ
│   ├── middleware/            # request_id / access_log / recover (D1 順序)
│   ├── server/                # *http.Server 構築 + ハンドラ登録 + middleware チェーン (M1.1 で OIDC route 統合)
│   ├── health/                # /health (M0.1) / /health/live / /health/ready (M0.3)
│   ├── db/                    # pgxpool 接続層 (Open / BuildDSN / SafeRepr)
│   ├── dbmigrate/             # golang-migrate ラッパ (RunUp / RunDown / AssertClean / ErrDirty)
│   ├── testutil/dbtest/       # 統合テスト DB ヘルパー (NewPool / BeginTx / RollbackTx)
│   ├── keystore/              # OIDC 署名鍵管理 (M1.1、KeySet I/F + staticKeySet + DeriveKid)
│   └── oidc/
│       ├── discovery/         # /.well-known/openid-configuration handler (M1.1、F-1〜F-4)
│       ├── jwks/              # /jwks handler (M1.1、F-5/F-6、jwx/v3 採用)
│       └── notimpl/           # /authorize, /token, /userinfo の 503 stub (M1.1、F-23)
├── bin/                       # ビルド成果物 (.gitignore で除外)
├── go.mod
└── Makefile
```

後続マイルストーン (M1.2-1.4) で `notimpl` の各 handler を本実装に差し替える。

## 環境変数

### サーバ / ロガー

| Key               | 既定値 | 範囲                 | 説明                                             |
| ----------------- | ------ | -------------------- | ------------------------------------------------ |
| `CORE_PORT`       | `8080` | `1〜65535` の整数    | HTTP サーバーのリッスンポート                    |
| `CORE_LOG_FORMAT` | `json` | `json` または `text` | ログ出力フォーマット (本番=`json` / 開発=`text`) |

### DB 接続 (M0.3 追加、起動時に必須)

| Key                | 既定値    | 範囲                                                                     | 説明                    |
| ------------------ | --------- | ------------------------------------------------------------------------ | ----------------------- |
| `CORE_DB_HOST`     | (必須)    | -                                                                        | PostgreSQL 接続先ホスト |
| `CORE_DB_PORT`     | (必須)    | 1〜65535                                                                 | PostgreSQL 接続ポート   |
| `CORE_DB_USER`     | (必須)    | -                                                                        | DB ユーザー             |
| `CORE_DB_PASSWORD` | (必須)    | -                                                                        | DB パスワード           |
| `CORE_DB_NAME`     | (必須)    | -                                                                        | DB 名                   |
| `CORE_DB_SSLMODE`  | `disable` | `disable` / `allow` / `prefer` / `require` / `verify-ca` / `verify-full` | SSL/TLS モード (Q10)    |

### DB プール (M0.3 追加、任意、未設定時は既定値)

| Key                                | 既定値 | 説明                         |
| ---------------------------------- | ------ | ---------------------------- |
| `CORE_DB_POOL_MAX_CONNS`           | `10`   | 最大接続数                   |
| `CORE_DB_POOL_MIN_CONNS`           | `1`    | 最小接続数                   |
| `CORE_DB_POOL_MAX_CONN_LIFETIME`   | `5m`   | コネクション最大生存時間     |
| `CORE_DB_POOL_MAX_CONN_IDLE_TIME`  | `2m`   | コネクション最大アイドル時間 |
| `CORE_DB_POOL_HEALTH_CHECK_PERIOD` | `30s`  | プールヘルスチェック周期     |

### マイグレーション / テスト (M0.3 追加、任意)

| Key                   | 既定値                                                        | 説明                                                                            |
| --------------------- | ------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| `CORE_MIGRATIONS_DIR` | `db/migrations` (cwd=core/ 前提の相対パス)                    | `dbmigrate.AssertClean` が参照するディレクトリ。CI / コンテナでは絶対パスを設定 |
| `TEST_DATABASE_URL`   | `postgres://core:core_dev_pw@localhost:5432/id_core_test?...` | 統合テスト用 DSN                                                                |
| `TEST_DB_REQUIRED`    | (未設定)                                                      | `1` を設定すると DB 接続失敗を skip ではなく fail にする (CI 用)                |

### 環境識別子 + OIDC (M1.1 追加)

| Key                                | 既定値   | 範囲                                                                         | 必須   | 説明                                                                                          |
| ---------------------------------- | -------- | ---------------------------------------------------------------------------- | ------ | --------------------------------------------------------------------------------------------- |
| `CORE_ENV`                         | (必須)   | `prod` / `staging` / `dev` (strict 3 値)                                     | 必須   | 環境識別子。それ以外で起動失敗。Makefile の `run` で `CORE_ENV ?= dev` がデフォルト           |
| `CORE_OIDC_ISSUER`                 | (必須)   | `https://...` (`prod`/`staging`)、`http://`/`https://` (`dev`)、末尾 / strip | 必須   | OP の論理識別子 URL                                                                           |
| `CORE_OIDC_KEY_FILE`               | (条件付) | PEM PKCS#8 秘密鍵ファイルパス                                                | 条件付 | `prod` で必須、`staging`/`dev` は `CORE_OIDC_DEV_GENERATE_KEY=1` で代替可                     |
| `CORE_OIDC_DEV_GENERATE_KEY`       | (任意)   | `0` または `1`                                                               | 任意   | `1` で起動時 RSA 2048 bit メモリ生成。`prod` では強制無効 (起動失敗)                          |
| `CORE_OIDC_KEY_ID`                 | (任意)   | 任意の文字列                                                                 | 任意   | kid 固定値。未設定時は keystore で公開鍵 DER SHA-256 先頭 24 hex を自動算出 (RFC 7638 非準拠) |
| `CORE_OIDC_JWKS_MAX_AGE`           | `300`    | 0〜86400 秒                                                                  | 任意   | JWKS Cache-Control max-age                                                                    |
| `CORE_OIDC_DISCOVERY_MAX_AGE`      | `0`      | 0〜86400 秒                                                                  | 任意   | Discovery Cache-Control max-age (0 → no-cache, must-revalidate / >0 → public, max-age=N)      |
| `CORE_OIDC_AUTHORIZATION_ENDPOINT` | (任意)   | 絶対 URL                                                                     | 任意   | 未設定なら issuer + `/authorize` (subpath 透過)                                               |
| `CORE_OIDC_TOKEN_ENDPOINT`         | (任意)   | 絶対 URL                                                                     | 任意   | 未設定なら issuer + `/token`                                                                  |
| `CORE_OIDC_USERINFO_ENDPOINT`      | (任意)   | 絶対 URL                                                                     | 任意   | 未設定なら issuer + `/userinfo`                                                               |
| `CORE_OIDC_JWKS_URI`               | (任意)   | 絶対 URL                                                                     | 任意   | 未設定なら issuer + `/jwks` (Q9: 拡張子なし)                                                  |

## ログ・エラー規約

`core/` のログとエラーレスポンスは構造化規約に従う:

- **ロガー**: `log/slog` ベース。`CORE_LOG_FORMAT=json` (既定) で JSON Lines、`text` で開発向け key=value
- **時刻**: `time` フィールドは RFC3339Nano UTC (`Z` suffix 強制、`time.Local` への副作用なし)
- **request_id**: 全 HTTP リクエストに UUID v7 で発番、レスポンスヘッダ `X-Request-Id` で返却
- **event_id**: 起動・signal handler・ジョブ等の非 HTTP 経路に UUID v7 を付与
- **エラーレスポンス**: `internal/apperror/` の基本形 `{ "code": "...", "message": "...", "details"?: {...}, "request_id": "..." }`
- **panic 時**: HTTP 500 + `{ "code": "INTERNAL_ERROR", ... }` のみ返し、スタックトレースは内部ログにのみ記録
- **redact**: 認可・トークン・PII 系のキー (例: `password` / `access_token` / `Authorization` 等) はログ出力前に `[REDACTED]` 固定値へ置換
- **DB 経路の F-10**: 接続失敗ログには `host` / `port` / `user` / `dbname` / `sslmode` のみ。password / DSN フルダンプは絶対に含めない (`db.SafeRepr` で生成)

詳細な規約は以下を参照:

- ロギング・テレメトリ / エラーハンドリング / middleware 構成 / DB / マイグレーション: [`docs/context/backend/conventions.md`](../docs/context/backend/conventions.md)
- 実装パターン (DB 接続 / マイグレーション運用 / 統合テスト / context ID 伝播): [`docs/context/backend/patterns.md`](../docs/context/backend/patterns.md)
- パッケージ・環境変数・エラーコード・マイグレーション一覧: [`docs/context/backend/registry.md`](../docs/context/backend/registry.md)
- テストパターン (DB を要するテスト / migrate 整合 / `/health/ready` テスト): [`docs/context/testing/backend.md`](../docs/context/testing/backend.md)

## クイックスタート

### 初回セットアップ

```bash
# DB の起動 (リポジトリルートから)
cp docker/.env.sample docker/.env       # 既に存在する場合はスキップ
docker compose -f docker/compose.yaml up -d postgres

# golang-migrate CLI のインストール (Go 経由)
make -C core migrate-install
# → $(go env GOPATH)/bin/migrate がインストールされる
# 必要なら PATH を通す: export PATH="$PATH:$(go env GOPATH)/bin"
```

### 開発フロー (M1.1 OIDC 起動を含む完全版)

```bash
# 1. dev 鍵を生成 (core/dev-keys/ 配下、.gitignore 除外)
make -C core dev-keygen
# → core/dev-keys/signing.pem (0600) + signing.pub.pem (0644) が生成される

# 2. マイグレーション適用 (開発 DB)
export CORE_DB_HOST=localhost CORE_DB_PORT=5432 CORE_DB_USER=idcore CORE_DB_PASSWORD=idcore CORE_DB_NAME=idcore CORE_DB_SSLMODE=disable
make -C core migrate-up

# 3. OIDC env を設定して起動 (CORE_ENV はデフォルト dev、CORE_OIDC_ISSUER + KEY_FILE 指定)
export CORE_OIDC_ISSUER=http://localhost:8080
export CORE_OIDC_KEY_FILE=./dev-keys/signing.pem
make -C core run
# → core/bin/core が起動し、:8080 で listen
# → 起動 INFO ログ: 起動鍵情報 (kid / alg=RS256 / source=file / env=dev)
```

または起動時鍵生成モード (`CORE_OIDC_KEY_FILE` の代わりに):

```bash
export CORE_OIDC_DEV_GENERATE_KEY=1  # メモリ生成、再起動で別鍵
# → 起動 INFO ログに加えて WARN: 「dev 鍵生成モード = 単一 Pod 専用、複数 Pod 環境では CORE_OIDC_KEY_FILE で共有 Secret を使え」
```

### 動作確認

別シェルで:

```bash
$ curl -i http://localhost:8080/health
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{"status":"ok"}

$ curl -i http://localhost:8080/health/live
HTTP/1.1 200 OK

{"status":"ok"}

$ curl -i http://localhost:8080/health/ready
HTTP/1.1 200 OK
# DB 接続が落ちると 503 + {"status":"unavailable"}
```

#### M1.1 OIDC エンドポイント

```bash
# Discovery (OIDC Discovery 1.0 / RFC 8414)
$ curl -i http://localhost:8080/.well-known/openid-configuration
HTTP/1.1 200 OK
Content-Type: application/json
Cache-Control: no-cache, must-revalidate
ETag: "<24 chars>"

{"issuer":"http://localhost:8080","authorization_endpoint":"http://localhost:8080/authorize",...}

# JWKS (RFC 7517)
$ curl -i http://localhost:8080/jwks
HTTP/1.1 200 OK
Content-Type: application/json
Cache-Control: public, max-age=300, must-revalidate
ETag: "<24 chars>"

{"keys":[{"alg":"RS256","e":"AQAB","kid":"<24 hex>","kty":"RSA","n":"<base64url>","use":"sig"}]}

# 503 stub (M1.2-1.4 で本実装、現状はメタデータと一致するための予告)
$ curl -i http://localhost:8080/authorize
HTTP/1.1 503 Service Unavailable
Content-Type: application/json
Cache-Control: no-store

{"error":"endpoint_not_implemented","available_at":"M1.2"}
```

#### `If-None-Match` で 304 Not Modified

Discovery / JWKS は決定的シリアライズ + strong ETag を返すため、RP の HTTP キャッシュが機能する:

```bash
$ curl -s -D - -o /dev/null -H 'If-None-Match: "<前回取得した ETag>"' http://localhost:8080/jwks
HTTP/1.1 304 Not Modified
ETag: "<同値>"
```

未対応メソッドは `405 Method Not Allowed` + `Allow` ヘッダを返す (例: `POST /authorize` は 405)。

### dirty 状態からの復旧

起動時に `dbmigrate: schema_migrations is dirty` で起動拒否された場合:

```bash
# 1. 現在の version を確認
make -C core migrate-version

# 2. 該当 version を強制リセット (例: VERSION=1)
make -C core migrate-force VERSION=1

# 3. 必要に応じてマイグレーションを再実行
make -C core migrate-up
```

## 主要コマンド

| コマンド                          | 用途                                                               |
| --------------------------------- | ------------------------------------------------------------------ |
| `make build`                      | バイナリをビルド (`bin/core`)                                      |
| `make run`                        | ビルド + 起動                                                      |
| `make test`                       | ユニットテスト (`go test -race ./...`、DB 不要)                    |
| `make test-integration`           | 統合テスト (DB 必要、build tag = `integration`、`-p 1` で順次実行) |
| `make test-cover`                 | カバレッジレポート生成                                             |
| `make lint`                       | `go vet ./...` + `log.Fatal*` 検査 + マイグレーション連番衝突検出  |
| `make clean`                      | ビルド成果物を削除                                                 |
| `make migrate-install`            | golang-migrate CLI v4.19.1 をインストール                          |
| `make migrate-create NAME=<slug>` | 雛形生成 (8 桁連番)                                                |
| `make migrate-up`                 | 全 pending を適用                                                  |
| `make migrate-up-one`             | 次の 1 件を適用                                                    |
| `make migrate-down`               | 直近 1 件をロールバック                                            |
| `make migrate-down-all`           | 全件ロールバック (危険、警告メッセージ付き)                        |
| `make migrate-force VERSION=<n>`  | dirty 状態の強制リセット                                           |
| `make migrate-version`            | 現在 version を表示                                                |
| `make migrate-status`             | graceful 表示 (no-version は exit 0、認証/接続エラーは通常 exit)   |
| `make dev-keygen`                 | dev 用 RSA 2048 bit 鍵を `core/dev-keys/` に生成 (M1.1、F-13)      |

## エンドポイント (M1.1 時点)

| Method | Path                                | 認証 | 状態        | 概要                                                         |
| ------ | ----------------------------------- | ---- | ----------- | ------------------------------------------------------------ |
| GET    | `/health`                           | 不要 | M0.1        | サーバー稼働確認 (後方互換、`{"status":"ok"}`)               |
| GET    | `/health/live`                      | 不要 | M0.3        | プロセス疎通 (DB 非依存、k8s livenessProbe 想定)             |
| GET    | `/health/ready`                     | 不要 | M0.3        | DB Ping with 2s timeout、200 / 503 (k8s readinessProbe 想定) |
| GET    | `/.well-known/openid-configuration` | 不要 | M1.1        | OIDC Discovery 1.0 / RFC 8414 メタデータ                     |
| GET    | `/jwks`                             | 不要 | M1.1        | JWKS (RFC 7517) — Q9: 拡張子なし                             |
| GET    | `/authorize`                        | 不要 | M1.1 (stub) | 503 + `{"available_at":"M1.2"}`、M1.2 で本実装               |
| POST   | `/token`                            | 不要 | M1.1 (stub) | 503 + `{"available_at":"M1.3"}`、M1.3 で本実装               |
| GET    | `/userinfo`                         | 不要 | M1.1 (stub) | 503 + `{"available_at":"M1.4"}`、M1.4 で本実装               |

## 関連ドキュメント

- 要求文書 (M0.1): [`docs/requirements/1/index.md`](../docs/requirements/1/index.md)
- 設計書 (M0.1): [`docs/specs/1/index.md`](../docs/specs/1/index.md)
- 要求文書 (M0.3): [`docs/requirements/21/index.md`](../docs/requirements/21/index.md)
- 設計書 (M0.3): [`docs/specs/21/index.md`](../docs/specs/21/index.md)
- 要求文書 (M1.1): [`docs/requirements/32/index.md`](../docs/requirements/32/index.md)
- 設計書 (M1.1): [`docs/specs/32/index.md`](../docs/specs/32/index.md)
- バックエンド規約: [`docs/context/backend/conventions.md`](../docs/context/backend/conventions.md)
- バックエンドパターン: [`docs/context/backend/patterns.md`](../docs/context/backend/patterns.md)
- バックエンドレジストリ: [`docs/context/backend/registry.md`](../docs/context/backend/registry.md)
- バックエンドテスト規約: [`docs/context/testing/backend.md`](../docs/context/testing/backend.md)
