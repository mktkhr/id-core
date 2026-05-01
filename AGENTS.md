Please also reference the following rules as needed. The list below is provided in TOON format, and `@` stands for the project root directory.

rules[6]{path}:
  @.codex/memories/authorization-matrix.md
  @.codex/memories/codex-delegation.md
  @.codex/memories/design-workflow.md
  @.codex/memories/issue-traceability.md
  @.codex/memories/path-resolution.md
  @.codex/memories/pr-review-policy.md

# id-core モノレポ — ルートルール

## プロジェクト概要

既存の OIDC IdP と複数プロダクトの間に立つ **ID 中間管理システム (OIDC OP)** の検証用モノレポ / リファレンス実装。

詳細は `docs/context/app/architecture.md` を参照。

## モノレポ構成

```
core/                     OIDC OP 本体 (Go)
examples/
  go-react/               React SPA + Go バックエンド
    backend/  frontend/
  kotlin-nextjs/          Next.js + Spring Boot (Kotlin) バックエンド
    backend/  frontend/
docs/                     spec-first ドキュメント
  context/                実装コンテキスト集約 (規約・パターン・registry・認可マトリクス正本)
  requirements/{N}/       要求文書
  specs/{N}/              設計書
  adr/                    Architecture Decision Records
  templates/              テンプレート
docker/                   開発環境 (compose.yaml)
.rulesync/                スキル・ルール正本 (本ディレクトリ)
アーカイブ/               参考資料 (.gitignore 対象、コミットしない)
```

## スキル・ルール管理 (rulesync)

`.rulesync/` 配下が**スキル・ルールの正本**。`make rulesync-generate` で各 AI ツール向けに展開される。

```
.rulesync/  ← 正本 (編集対象)
   │
   │ make rulesync-generate
   ▼
CLAUDE.md (このファイル) / .claude/skills/ ...   ← 生成物 (直接編集禁止)
AGENTS.md / .codex/ ...                         ← 生成物 (Codex 向け、必要時)
.clinerules ...                                 ← 生成物 (Cline 向け、必要時)
```

- スキルやルールを編集する場合は **`.rulesync/` 側を編集**してから `make rulesync`
- このファイル (`CLAUDE.md`) を直接編集してはならない (rulesync 再生成時に上書きされる)

## 認可マトリクス運用

`docs/context/authorization/matrix.md` が**認可の唯一の正本**。
設計書 (`docs/specs/*/index.md`) の認可表は参照コピーであり、独自判断で拡大/縮小してはならない。
マスターと差分を検知した場合、全ての spec 系スキルは**即停止してユーザー判断を仰ぐ**。

詳細は `.rulesync/rules/authorization-matrix.md` を参照。

## 認証プロトコル方針

- **上流 (id-core ← 上流 IdP)**: id-core は **RP** として OIDC で連携
- **下流 (id-core → サンプルアプリ)**: id-core は **OIDC OP** として振る舞う
- IAM ミドルウェア (Keycloak / Hydra / Authelia / fosite 等) は**不採用**
- Go パッケージ層は積極利用 (go-oidc / lestrrat-go/jwx 等)
- 暗号プリミティブは標準ライブラリ / 実績ライブラリ。自作禁止

## コミット規約

```
{type}:{emoji} {対象の説明}
```

| Type | 用途 | Emoji |
|---|---|---|
| feat | 新機能 | ✨ |
| fix | バグ修正 | 🐛 |
| docs | ドキュメント・設計書・ADR・README | 📝 |
| refactor | リネーム・構造変更 (内容変更なし) | ♻️ |
| chore | 設定・hooks・gitignore 等 | 🔧 |
| test | テスト追加・修正 | ✅ |

詳細は `.rulesync/skills/commit/SKILL.md` (生成後は `/commit` スキル) を参照。

## パス解決

このリポジトリは**モノレポ**。すべてのパスは**リポジトリルートからの相対パス**で解決する。
個人 PC の絶対パス (`/Users/xxx/`, `/home/xxx/` 等) のハードコード禁止。
詳細は `.rulesync/rules/path-resolution.md`。

## スコープ管理

このリポジトリは**検証用 PoC / リファレンス実装**。

- 設計品質は本番水準を意識する (使い捨てではない)
- 本番運用そのものはこのリポジトリの範囲外
- 設計・スキルは「id-core 本体 + 動作確認用サンプル × 2」の検証範囲に集中する。スコープ外の機能を持ち込まない

## アーカイブ/ について

`アーカイブ/` 配下に AI-first / spec-first を実践した参考リポジトリが展開済み。
ワークフロー・規約のひな型として参照可。**ただしそのドメインロジックは id-core に持ち込まない**。

スキル・ルール・コードがアーカイブを直接参照することは禁止 (汎用的に動作させるため)。
ユーザーが明示的に求めた場合のみ Read で内容確認する。
