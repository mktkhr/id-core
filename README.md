# id-core

既存の OIDC IdP と複数プロダクトの間に立つ **ID 中間管理システム** の検証用モノレポ。

## 目的

OIDC 対応の上流 IdP を委譲先に置きつつ、上流 IdP に欠けている認証手段 (例: 電話番号 / LINE 等の SNS ログイン) を補い、複数プロダクト間で共通の User ID を発行する **OIDC OP** のリファレンス実装。

「既存の IdP は使えるが、自プロダクト群の要件 (追加認証手段・横断 ID) には足りない」というよくある状況に対する汎用テンプレートを目指す。

このリポジトリは PoC / 動作確認用であり、本番運用するシステムを直接ホストするものではない。

## 構成

```
core/                     OIDC OP 本体 (Go)
examples/
  go-react/               React SPA + Go バックエンド
    backend/
    frontend/
  kotlin-nextjs/          Next.js + Spring Boot (Kotlin) バックエンド
    backend/
    frontend/
docs/                     spec-first ドキュメント
docker/                   開発環境 (compose.yaml + Dockerfile 群)
```

サンプル 2 つで「Go / Kotlin」「SPA / SSR」の合計 4 軸を検証する設計。

## セットアップ (clone 直後に 1 回)

```bash
make dev-init       # docker/.env を docker/.env.sample からコピー
make hooks-install  # pre-commit hook を有効化 (Markdown を自動整形)
```

## 開発

```bash
make dev-up         # docker compose 経由で開発環境を起動
make dev-down       # 停止
make dev-logs       # コンテナログを追従

make format         # prettier で Markdown 等を整形
make rulesync       # .rulesync/ から各 AI ツール向けにスキル・ルールを生成
```

詳細コマンドは `make help`。

## ドキュメント

詳細は `docs/README.md` および `docs/context/app/architecture.md` を参照。
