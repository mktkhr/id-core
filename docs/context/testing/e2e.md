# E2E テスト規約

> 最終更新: 2026-04-25 (骨格セットアップ時点、内容 TBD)
> このファイルは実装が始まった段階で記述する。

## ツール選定

TBD — Playwright を想定するが、各サンプルアプリの実装時に確定する。

## サンプルアプリ別の検証観点

### examples/go-react (React SPA + Go)

TBD — Authorization Code + PKCE フロー、トークンリフレッシュ、ログアウトの検証。

### examples/kotlin-nextjs (Next.js + Spring Boot)

TBD — BFF / SSR フロー、Cookie セッション、ログアウトの検証。

## 共通検証観点 (id-core 側)

TBD — 上流 IdP 委譲フロー、電話番号認証フロー、SNS 認証フロー、アカウントリンクの検証。

## data-testid 規約

TBD
