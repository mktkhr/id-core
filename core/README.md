# core/ — id-core OIDC OP 本体

id-core の OIDC OP (Identity Provider) 本体の Go 実装。

## 前提

- **Go**: `1.22+` (動作確認: `1.26.2`)
- POSIX 準拠シェル + `make`

## ディレクトリ構成 (M0.2 時点)

```
core/
├── cmd/core/main.go           # エントリポイント (起動・signal handling)
├── internal/
│   ├── config/                # 環境変数読み込み + バリデーション
│   ├── logger/                # 構造化ロガー (log/slog ラッパ + redact + fallback)
│   ├── apperror/              # 構造化エラー型 + JSON シリアライザ
│   ├── middleware/            # request_id / access_log / recover (D1 順序)
│   ├── server/                # *http.Server 構築 + ハンドラ登録 + middleware チェーン
│   └── health/                # /health エンドポイント
├── bin/                       # ビルド成果物 (.gitignore で除外)
├── go.mod
└── Makefile
```

後続マイルストーン (M0.3 DB / M1.x OIDC) でレイヤが拡張される。

## 環境変数

| Key               | 既定値 | 範囲                 | 説明                                             |
| ----------------- | ------ | -------------------- | ------------------------------------------------ |
| `CORE_PORT`       | `8080` | `1〜65535` の整数    | HTTP サーバーのリッスンポート                    |
| `CORE_LOG_FORMAT` | `json` | `json` または `text` | ログ出力フォーマット (本番=`json` / 開発=`text`) |

未設定時は既定値で起動。不正値 (非数値・範囲外、`CORE_LOG_FORMAT` の許容外値) の場合は明示エラーで起動失敗する。

## ログ・エラー規約

`core/` のログとエラーレスポンスは構造化規約に従う:

- **ロガー**: `log/slog` ベース。`CORE_LOG_FORMAT=json` (既定) で JSON Lines、`text` で開発向け key=value
- **時刻**: `time` フィールドは RFC3339Nano UTC (`Z` suffix 強制、`time.Local` への副作用なし)
- **request_id**: 全 HTTP リクエストに UUID v7 で発番、レスポンスヘッダ `X-Request-Id` で返却
- **event_id**: 起動・signal handler・ジョブ等の非 HTTP 経路に UUID v7 を付与
- **エラーレスポンス**: `internal/apperror/` の基本形 `{ "code": "...", "message": "...", "details"?: {...}, "request_id": "..." }` (JSON、`code` は `SCREAMING_SNAKE_CASE`、`details` は任意)
- **panic 時**: HTTP 500 + `{ "code": "INTERNAL_ERROR", "message": "...", "request_id": "..." }` のみ返し、スタックトレースは内部ログにのみ記録
- **redact**: 認可・トークン・PII 系のキー (例: `password` / `access_token` / `Authorization` 等) はログ出力前に `[REDACTED]` 固定値へ置換

詳細な規約は以下を参照:

- ロギング・テレメトリ / エラーハンドリング / middleware 構成: [`docs/context/backend/conventions.md`](../docs/context/backend/conventions.md)
- 実装パターン (middleware チェーン / context ID 付与 / redact): [`docs/context/backend/patterns.md`](../docs/context/backend/patterns.md)
- パッケージ・環境変数・エラーコード: [`docs/context/backend/registry.md`](../docs/context/backend/registry.md)

## クイックスタート

```bash
# ビルド
make build
# → core/bin/core が生成される

# 起動 (デフォルトポート 8080)
make run
# または環境変数指定
CORE_PORT=9090 ./bin/core
```

### 動作確認

別シェルで:

```bash
$ curl -i http://localhost:8080/health
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
Content-Length: 16

{"status":"ok"}
```

未対応メソッドは `405 Method Not Allowed` + `Allow: GET, HEAD` ヘッダを返す:

```bash
$ curl -i -X POST http://localhost:8080/health
HTTP/1.1 405 Method Not Allowed
Allow: GET, HEAD
```

## 主要コマンド

| コマンド          | 用途                                   |
| ----------------- | -------------------------------------- |
| `make build`      | バイナリをビルド (`bin/core`)          |
| `make run`        | ビルド + 起動                          |
| `make test`       | ユニットテスト (`go test -race ./...`) |
| `make test-cover` | カバレッジレポート生成                 |
| `make lint`       | `go vet ./...` + `log.Fatal*` 検査     |
| `make clean`      | ビルド成果物を削除                     |

## エンドポイント (M0.1 時点)

| Method | Path      | 認証 | 概要                                 |
| ------ | --------- | ---- | ------------------------------------ |
| GET    | `/health` | 不要 | サーバー稼働確認 (`{"status":"ok"}`) |

OIDC 標準エンドポイント (`/authorize`, `/token`, `/userinfo`, `/jwks`, `/.well-known/openid-configuration` 等) は M1.x で順次追加。

## 関連ドキュメント

- 要求文書: [`docs/requirements/1/index.md`](../docs/requirements/1/index.md)
- 設計書: [`docs/specs/1/index.md`](../docs/specs/1/index.md)
- バックエンド規約: [`docs/context/backend/conventions.md`](../docs/context/backend/conventions.md)
- バックエンドパターン: [`docs/context/backend/patterns.md`](../docs/context/backend/patterns.md)
- バックエンドレジストリ: [`docs/context/backend/registry.md`](../docs/context/backend/registry.md)
