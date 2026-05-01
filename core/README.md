# core/ — id-core OIDC OP 本体

id-core の OIDC OP (Identity Provider) 本体の Go 実装。

## 前提

- **Go**: `1.22+` (動作確認: `1.26.2`)
- POSIX 準拠シェル + `make`

## ディレクトリ構成 (M0.1 時点)

```
core/
├── cmd/core/main.go           # エントリポイント
├── internal/
│   ├── config/                # 環境変数読み込み + バリデーション
│   ├── server/                # *http.Server 構築 + ハンドラ登録
│   └── health/                # /health エンドポイント
├── bin/                       # ビルド成果物 (.gitignore で除外)
├── go.mod
└── Makefile
```

後続マイルストーン (M0.2 ログ規約 / M0.3 DB / M1.x OIDC) でレイヤが拡張される。

## 環境変数

| Key         | 既定値 | 範囲              | 説明                          |
| ----------- | ------ | ----------------- | ----------------------------- |
| `CORE_PORT` | `8080` | `1〜65535` の整数 | HTTP サーバーのリッスンポート |

未設定時は既定値で起動。不正値 (非数値・範囲外) の場合は明示エラーで起動失敗する。

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
| `make lint`       | `go vet ./...`                         |
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
