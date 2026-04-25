---
name: commit
description: '変更を適切な粒度に分割してコミットする。"コミット", "commit", "変更をコミット" 等で発動。'
targets:
  - "*"
---

# スマートコミット

変更を関心事ごとに分割し、規約に従ったコミットメッセージで個別にコミットする。

## コミットメッセージ規約

```
{type}:{emoji} {対象の説明}
```

- subject 1 行のみ。body (詳細説明) は書かない
- 日本語で記載
- "Generated with Claude Code" / "Co-Authored-By: Claude" を含めない

### Type + Emoji

| Type | 用途 | Emoji |
|---|---|---|
| feat | 新機能 | ✨ |
| fix | バグ修正・誤記修正・パス修正・不整合修正 | 🐛 |
| docs | 設計書・テンプレート・スキル・ADR・README・context | 📝 |
| refactor | リネーム・構造変更 (内容変更なし) | ♻️ |
| chore | 設定・hooks・gitignore・Makefile 等 | 🔧 |
| test | テスト追加・修正 | ✅ |

使用頻度低 (必要に応じて):

| Type | 用途 | Emoji |
|---|---|---|
| add | 新規ファイル追加 (機能追加に該当しないとき) | ✨ |
| remove | ファイル削除 | 🗑️ |

## 手順

1. `git status` と `git diff` で全変更を把握する
2. `git log --oneline -10` で直近のコミットメッセージパターンを確認する
3. 変更を関心事ごとにグループ分けする
4. グループごとに:
   - 対象ファイルのみ `git add` でステージング
   - `git diff --cached` で差分を確認
   - コミットメッセージでコミット
5. 全コミット完了後、`git log --oneline` で結果を表示する

## 分割基準

| 関心事 | 例 |
|---|---|
| id-core 本体 | `core/cmd/`, `core/internal/`, `core/db/`, `core/api/` 等 |
| サンプル backend (Go) | `examples/go-react/backend/` |
| サンプル backend (Kotlin) | `examples/kotlin-nextjs/backend/` |
| サンプル frontend (React) | `examples/go-react/frontend/` |
| サンプル frontend (Next.js) | `examples/kotlin-nextjs/frontend/` |
| 設計書本体 | `docs/specs/{N}/index.md` |
| 実装プロンプト | `docs/specs/{N}/prompts/` |
| 要求文書 | `docs/requirements/{N}/` |
| 認可マトリクス更新 | `docs/context/authorization/matrix.md` |
| context (規約・パターン・registry) | `docs/context/{backend,frontend,testing}/*.md` |
| アーキテクチャ | `docs/context/app/architecture.md` |
| テンプレート | `docs/templates/` |
| スキル・ルール (rulesync 正本) | `.rulesync/skills/`, `.rulesync/rules/` |
| Claude Code 生成物 | `.claude/` (rulesync で生成された場合) |
| ADR | `docs/adr/` |
| README / CLAUDE.md | 各ファイル |
| docker / Makefile | `docker/`, `Makefile` |

ユーザーから「めっちゃ細かく」等の指示があればそれに従う。

## 禁止事項

- `git add -A` / `git add .` は使わない (意図しないファイルを含むリスク)
- 1 つのコミットに全変更をまとめない
- `.env`, credentials, トークン等を含むファイルはコミットしない
- `アーカイブ/` 配下はコミットしない (`.gitignore` 済み)
- `.ai-out/` 配下があればコミットしない
- ユーザーが明示的に指示しない限り、`git push` / `--force` / `--amend` / `reset --hard` は実行しない
