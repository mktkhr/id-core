# バックエンド規約 (id-core / Go)

> 最終更新: 2026-05-02 (M0.1: HTTP サーバー骨格 反映)

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

| ターゲット   | 用途                                                  |
| ------------ | ----------------------------------------------------- |
| `help`       | ターゲット一覧を表示 (`make` または `make help`)      |
| `build`      | バイナリをビルド (`go build -o bin/core ./cmd/core`)  |
| `run`        | ビルド + 起動                                         |
| `test`       | `go test -race ./...`                                 |
| `test-cover` | カバレッジ計測 (`-coverprofile=coverage.txt`)         |
| `lint`       | `go vet ./...` (M0.2 以降で `golangci-lint` 導入予定) |
| `clean`      | `bin/` と `coverage.txt` を削除                       |

## 環境変数読み込みパターン

- 環境変数は `internal/config/config.go` の `Load()` で集約読み込み
- バリデーションエラーは `error` で返す (`log.Fatal` を直接呼ばない → テスト容易性確保)
- `cmd/<binary>/main.go` で `error` を受けて `log.Fatalf` で異常終了
- 命名: `CORE_<NAME>` プレフィックス (例: `CORE_PORT`)
- 範囲制約があるものは `MinXxx` / `MaxXxx` 定数として `config` パッケージで宣言

## DB / マイグレーション

TBD (M0.3 で確定)

## API (OIDC OP / 標準エンドポイント)

TBD — `/authorize`, `/token`, `/userinfo`, `/jwks.json`, `/.well-known/openid-configuration` の規約 (M1.x で確定)

### 既存エンドポイント

| Path      | Method | 認証 | 概要                                 |
| --------- | ------ | ---- | ------------------------------------ |
| `/health` | GET    | 不要 | サーバー稼働確認 (`{"status":"ok"}`) |

## API (id-core 独自管理 API)

TBD — 内部ユーザー管理・アカウントリンク・電話番号認証・SNS 認証 (LINE 等) のエンドポイント規約

## エラーコード

TBD (M0.2 で確定)

## 認可

TBD — id-core 自身の管理 API の認可方式 (CLAUDE.md 方針: IAM ミドルウェア不採用、手書きポリシー)

## ロギング・テレメトリ

M0.1 暫定: 標準 `log` パッケージで `Printf` / `Fatalf` を使用。
M0.2 で `log/slog` ベースの構造化ログ + `request_id` ミドルウェアに置換する (`backend-logging` スキル参照)。
