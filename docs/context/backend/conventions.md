# バックエンド規約 (id-core / Go)

> 最終更新: 2026-04-25 (骨格セットアップ時点、内容 TBD)
> このファイルは id-core 本体の Go 実装が始まった段階で記述する。

## DB / マイグレーション

TBD

## API (OIDC OP / 標準エンドポイント)

TBD — `/authorize`, `/token`, `/userinfo`, `/jwks.json`, `/.well-known/openid-configuration` の規約。

## API (id-core 独自管理 API)

TBD — 内部ユーザー管理・アカウントリンク・電話番号認証・SNS 認証 (LINE 等) のエンドポイント規約。

## エラーコード

TBD

## 認可

TBD — id-core 自身の管理 API の認可方式 (Casbin 採用するか、別方式か含めて検討)。

## ロギング・テレメトリ

TBD
