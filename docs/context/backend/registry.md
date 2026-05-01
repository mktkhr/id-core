# バックエンドレジストリ (id-core / Go)

> 最終更新: 2026-05-02 (M0.1 反映)

## パッケージマッピング

| パス                   | 用途                                  | 依存                                             |
| ---------------------- | ------------------------------------- | ------------------------------------------------ |
| `core/cmd/core`        | 実行ファイルエントリポイント (`main`) | `internal/config`, `internal/server`, `log`      |
| `core/internal/config` | 環境変数読み込み + バリデーション     | `os`, `strconv`, `fmt`                           |
| `core/internal/server` | `*http.Server` 構築 + ハンドラ登録    | `internal/config`, `internal/health`, `net/http` |
| `core/internal/health` | `/health` ハンドラ                    | `encoding/json`, `net/http`                      |

## Feature ディレクトリマッピング

M0.1 では feature パッケージなし (機能横断的な骨格のみ)。
M1.x の OIDC 実装で `core/internal/features/<feature>/` が登場する。

## テーブル一覧

TBD (M0.3 で確定)

### マスターテーブル

TBD

### エンティティテーブル

TBD

### 中間テーブル

TBD

## API エンドポイント一覧

### OIDC 標準エンドポイント

TBD — `/authorize`, `/token`, `/userinfo`, `/jwks.json`, `/.well-known/openid-configuration` 等 (M1.x で確定)

### id-core 管理 API

TBD

### 共通

| Path      | Method | 認証 | 担当パッケージ    | 追加マイルストーン |
| --------- | ------ | ---- | ----------------- | ------------------ |
| `/health` | GET    | 不要 | `internal/health` | M0.1               |

## 環境変数一覧

| Key         | 既定値 | 範囲              | 必須 | 説明                               |
| ----------- | ------ | ----------------- | ---- | ---------------------------------- |
| `CORE_PORT` | `8080` | `1〜65535` の整数 | 任意 | core HTTP サーバーのリッスンポート |

## エラーコード一覧

TBD (M0.2 で確定)

## マイグレーション一覧

TBD (M0.3 で確定)
