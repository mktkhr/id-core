# バックエンドレジストリ (id-core / Go)

> 最終更新: 2026-05-02 (M0.2: ログ・エラー・middleware 反映)

## パッケージマッピング

| パス                       | 用途                                                                                      | 依存                                                                                        |
| -------------------------- | ----------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| `core/cmd/core`            | 実行ファイルエントリポイント (`main`)。起動・signal handling・最終 ID 発番・logger 初期化 | `internal/config`, `internal/logger`, `internal/server`, `os`, `os/signal`, `context`       |
| `core/internal/config`     | 環境変数読み込み + バリデーション                                                         | `os`, `strconv`, `fmt`                                                                      |
| `core/internal/logger`     | 構造化ロガー (slog ラッパ + Format / Context / Redact / FallbackWriter)                   | `log/slog`, `github.com/google/uuid`, `context`, `io`, `sync/atomic`                        |
| `core/internal/apperror`   | 構造化エラー型 (`CodedError`) + JSON シリアライザ (`Response` / `WriteJSON`)              | `encoding/json`, `errors`, `fmt`, `net/http`                                                |
| `core/internal/middleware` | request_id / access_log / recover (D1 順序の HTTP middleware 群)                          | `internal/logger`, `internal/apperror`, `github.com/google/uuid`, `net/http`, `sync/atomic` |
| `core/internal/server`     | `*http.Server` 構築 + ハンドラ登録 + middleware チェーン組み込み                          | `internal/config`, `internal/health`, `internal/logger`, `internal/middleware`, `net/http`  |
| `core/internal/health`     | `/health` ハンドラ (logger 受け取りに対応)                                                | `encoding/json`, `internal/logger`, `net/http`                                              |

## Feature ディレクトリマッピング

M0.2 では feature パッケージなし (機能横断的な骨格 + ログ・エラー基盤のみ)。
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

| Key               | 既定値 | 範囲                 | 必須 | 説明                                                               |
| ----------------- | ------ | -------------------- | ---- | ------------------------------------------------------------------ |
| `CORE_PORT`       | `8080` | `1〜65535` の整数    | 任意 | core HTTP サーバーのリッスンポート                                 |
| `CORE_LOG_FORMAT` | `json` | `json` または `text` | 任意 | ログ出力フォーマット (本番=`json` / 開発=`text`)。不正値で起動失敗 |

## エラーコード一覧

### 共通 (内部 API)

| code             | HTTP status | 用途                                                                        | 追加マイルストーン |
| ---------------- | ----------- | --------------------------------------------------------------------------- | ------------------ |
| `INTERNAL_ERROR` | 500         | panic 等の予期しないエラー (`recover` middleware が固定で返す / F-9 / F-10) | M0.2               |

### OIDC 標準エンドポイント

OIDC 標準エンドポイント (M1.x 以降) は RFC 6749 / 6750 / OpenID Connect Core が定める標準コード (`invalid_request` / `invalid_grant` / `invalid_token` / `invalid_client` 等) を採用する。本レジストリの `SCREAMING_SNAKE_CASE` は内部 API スコープに適用し、OIDC 標準コードは仕様準拠を優先する。

詳細な命名規則 / `details` 制約 / panic 時の挙動は `docs/context/backend/conventions.md` の「エラーハンドリング」節を参照。

## マイグレーション一覧

TBD (M0.3 で確定)
