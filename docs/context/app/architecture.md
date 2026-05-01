# id-core アーキテクチャ

> 最終更新: 2026-04-25 (骨格セットアップ時点)

## プロダクト概要

既存の OIDC IdP と複数プロダクトの間に立つ **ID 中間管理システム** のリファレンス実装。

想定する典型シナリオ:

- 既存の OIDC 対応 IdP は使えるが、**特定の認証手段に対応していない** (例: 電話番号認証、SNS ログイン)
- 上記を IdP 側で追加するのが (技術的または組織的に) 困難
- 複数プロダクトに跨る**共通 User ID**を中央管理したい
- 各プロダクトが個別に上流 IdP との連携と追加認証手段を実装する**重複を解消**したい

id-core はこれらを 1 箇所に集約し、**複数プロダクト間で共通の User ID** を発行・中央管理する **OIDC OP (OpenID Provider)** として振る舞う。

## このリポジトリの位置づけ

### 形態

- **検証用モノレポ (PoC) / リファレンス実装**
- このリポジトリ内では**すべて新規実装**で、id-core 自体と消費者となるサンプルアプリの両方を含める
- 設計・実装パターンは**本番水準を意識**する (検証だけが目的の使い捨てではない)
- 個人プロジェクトとして公開している。同様のシナリオ (既存 IdP + 認証手段補完 + 共通 ID 発行) を構築するチームの参考になることを目指す

### このリポジトリで検証したいこと (PoC ゴール)

1. **OIDC OP として動くか** — 標準的な OIDC クライアントから接続でき、Authorization Code + PKCE フローが完結する
2. **言語非依存に消費できるか** — Go / Kotlin の 2 種類の RP から問題なく接続できる
3. **消費モデル非依存に動くか** — SPA (React) と SSR (Next.js) の両方の OIDC クライアントパターンで動く
4. **電話番号 / SNS 認証を上流 IdP と並列に提供できるか** — 上流 IdP の OIDC フローと並べて違和感なく統合できる
5. **共通 User ID がプロダクト横断で機能するか** — 同一人物が異なる認証経路で入っても同じ ID として認識される (アカウントリンク)

### スコープ外 (このリポジトリでは扱わない)

- **本番運用そのもの** — このリポジトリは検証用
- **既存の運用システムからの段階移行設計** — 本番化時に別途検討
- **ユーザー登録・パスワードリセット等** — 上流 IdP の責務に属するため id-core では実装しない
- **各プロダクトの業務ロジック** — RP (サンプルアプリ) 側で実装する

## モノレポ構成

| パス                               | 役割                            | 技術スタック         |
| ---------------------------------- | ------------------------------- | -------------------- |
| `core/`                            | OIDC OP 本体                    | Go                   |
| `examples/go-react/backend/`       | サンプル A バックエンド (RP)    | Go                   |
| `examples/go-react/frontend/`      | サンプル A フロントエンド       | React (SPA)          |
| `examples/kotlin-nextjs/backend/`  | サンプル B バックエンド (RP)    | Spring Boot (Kotlin) |
| `examples/kotlin-nextjs/frontend/` | サンプル B フロントエンド       | Next.js (SSR / BFF)  |
| `docs/`                            | 設計書・要件・ADR・コンテキスト | Markdown             |
| `docker/`                          | 開発環境                        | docker compose       |

サンプル 2 つで「Go / Kotlin」「SPA / SSR」の **計 4 軸**を検証する設計。
id-core が言語非依存・消費モデル非依存で動くことを最小コストで確認する。

## プロトコル方針

### 上流 (id-core ← 上流 IdP)

- id-core は **RP (Relying Party)** として OIDC で上流 IdP に委譲
- ユーザー認証 (パスワード / MFA / アカウント回復) は上流 IdP に任せる
- id-core は ID トークン / アクセストークンを受け取り、内部ユーザー情報と紐付ける

### 下流 (id-core → サンプルアプリ)

- id-core は **OP (OpenID Provider)** として OIDC を提供
- 自社プロダクトしか乗らない想定だが、**既存の標準プロトコルに乗ることを優先**
- 標準エンドポイント: `/authorize`, `/token`, `/userinfo`, `/jwks.json`, `/.well-known/openid-configuration`
- フロー: Authorization Code + PKCE のみ。Implicit / Resource Owner Password は不採用

## 主要責務 (id-core が担うもの)

1. **上流 IdP の OIDC クライアント** — 既存の OIDC IdP に本人確認を委譲
2. **電話番号認証** — SMS OTP 等 (上流 IdP に欠けている穴を埋める例)
3. **SNS 認証** — LINE / その他 SNS ログイン API との連携 (上流 IdP に欠けている穴を埋める例)
4. **内部ユーザー DB と共通 User ID 発行**
5. **アカウントリンク** — 同一人物の複数 IdP / 認証手段紐付け
6. **プロダクト向け OIDC OP エンドポイント**
7. **認証セッション管理** (id-core 自身のセッション)

### id-core が担わないもの

- 各プロダクトのアプリセッション (プロダクト責務)
- プロダクト固有の認可 (プロダクト側で claims を見て決める)
- プロダクト固有のユーザープロファイル拡張

## OSS 利用方針

- **IAM ミドルウェア (Keycloak / Hydra / Authelia / fosite 等) は不採用**
  - id-core 相当のシステムを自前実装する想定。ミドルウェア OSS の設計制約に縛られたくない
  - 電話番号 / SNS 認証等の上流 IdP 補完カスタム要件を実装しやすくする
- **Go パッケージ層は積極利用**
  - OIDC クライアント: `github.com/coreos/go-oidc/v3`
  - JWT/JWS/JWK: `github.com/lestrrat-go/jwx/v2` (or `golang-jwt/jwt`)
  - 暗号: 標準ライブラリ (`crypto/*`)
- **暗号プリミティブの自作は禁止**

## 技術スタック決定事項 (現時点)

| 項目         | 選択                                   | 備考                      |
| ------------ | -------------------------------------- | ------------------------- |
| id-core 言語 | Go                                     | 確定                      |
| サンプル A   | React SPA + Go                         | 確定 (前後分離パターン)   |
| サンプル B   | Next.js + Kotlin/Spring Boot           | 確定 (BFF / SSR パターン) |
| 開発環境     | docker compose (`docker/compose.yaml`) | 確定                      |

## TBD (今後の設計フェーズで決定)

- DB 製品 (PostgreSQL を想定するが要確定)
- セッションストア (Redis / Valkey / DB / JWT セッション)
- Web フレームワーク (Gin / Echo / chi / 標準 net/http)
- ロギング・テレメトリ
- マイグレーションツール
- 鍵管理戦略 (JWKS rotation の実装方法)
- MFA 方針 (MVP に含めるか後置か)
