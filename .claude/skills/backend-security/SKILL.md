---
name: backend-security
description: >-
  バックエンドのセキュリティガイド。OIDC OP として/RP としての両側面を網羅。"backend-security", "セキュリティ確認",
  "認証実装" 等で発動。
---
# Backend Security Guide

id-core はバックエンドが **OIDC OP (下流)** + **OIDC RP (上流)** を兼ねる。
両側面のセキュリティ要件を満たすこと。

## 認証プロトコル方針 (前提)

- **上流 (id-core ← 上流 IdP)**: id-core は **RP**
- **下流 (id-core → サンプルアプリ)**: id-core は **OIDC OP**
- **IAM ミドルウェア (Keycloak / Hydra / Authelia / fosite 等) は不採用**
- Go パッケージ層は積極利用 (go-oidc / lestrrat-go/jwx 等)
- 暗号プリミティブは標準ライブラリまたは実績ライブラリ。**自作禁止**

## OIDC OP (下流) 実装の必須要件

- `state`: 認可リクエストごとに生成・セッション保管・コールバックで照合
- `nonce`: ID トークンに含めて再生時検証
- **PKCE 必須化** (public client / SPA は code_verifier / S256 必須)
- `redirect_uri`: クライアント登録時の完全一致 (前方一致禁止)
- **認可コードは 1 回限り** (使用後即時無効化)
- リフレッシュトークン rotation (使用済み即無効、不正使用検知時は family 全失効)
- ID トークン署名鍵: JWKS 公開、定期 rotation
- consent 画面の偽装防止 (CSRF token / Origin 検証)

## OIDC RP (上流) 実装の必須要件

- 上流 IdP の JWKS を取得し ID トークン検証 (`iss`, `aud`, `exp`, 署名)
- `state` / `nonce` 検証
- `redirect_uri` を environment ごとに固定
- アクセストークンは長期保管しない (短命 + refresh)

## 入力検証

すべての入力経路 (API) で統一バリデーション:

| 層 | 責務 |
|---|---|
| Presentation | フィールド形式・文字数・必須・型 (`apperror.BadRequestError()`) |
| UseCase | ビジネスルール (重複・存在・権限・状態遷移) |
| Repository | 制約違反のキャッチとログ + エラー返却 |

## シークレット管理

- 環境変数で管理 (`.env` を `docker/.env` に集約、リポジトリにコミットしない)
- 本番: 環境変数 / KMS 等で暗号化
- **以下はハードコード禁止**: クライアントシークレット、JWT 署名鍵、DB パスワード、上流 IdP のクライアント認証情報、JWKS エンドポイント URL

## 認可 (RBAC)

- **正本**: `docs/context/authorization/matrix.md` (画面機能 × ロールセット)
- 実装方式: id-core は IAM ミドルウェア不採用。**手書き認可ポリシー** (middleware で OIDC scope / role claim を検証し、UseCase 層で所有権チェック)
- ▲ (条件付き許可) は middleware で API アクセスを通過させ、UseCase で `userID == owner` 等のフィルタを実装
- エンドポイント追加時は必ず認可マトリクス正本と認可ポリシー実装を**同期更新** (差分検出時はユーザー承認必須、`.rulesync/rules/authorization-matrix.md` 参照)

## チェックリスト

- [ ] ハードコードされたシークレットがない
- [ ] ユーザー入力がバリデーションされている
- [ ] SQL インジェクション対策 (sqlc 等のパラメタライズドクエリ使用)
- [ ] エラーメッセージに機密情報 (内部パス、SQL、トークン) を含めない
- [ ] ログに機密情報 (token, password, ID トークン全文) を出力しない
- [ ] OIDC のフロー検証 (state / nonce / PKCE / redirect_uri)
- [ ] セッション固定攻撃対策 (ログイン後にセッション ID 再生成)
- [ ] 認可マトリクス正本と一致した認可実装
- [ ] CORS / CSRF 対策 (consent 画面など state 変更系)

## スキャンツール

```bash
# Go
gosec ./...
govulncheck ./...

# Kotlin
./gradlew dependencyCheckAnalyze
```
