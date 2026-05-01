# Issue トレーサビリティルール

## 原則

要求 → 設計 → 実装 → 検証の全フローを **GitHub Issue / PR のリンク**で追跡可能にする。
このリポジトリは**モノレポ**のため、リンクは `#番号` 形式で十分 (フルパス不要)。

## フロー

```
要求 Issue (#N: 機能/バグ/改善)
  → docs/requirements/{N}/index.md  (要求文書)
  → /requirements-publish で Issue を「設計着手OK」に
  → docs/specs/{N}/index.md         (設計書)
  → /issue-from-spec で実装タスク Issue を分割起票:
      ├─ #X 【実装】{機能}: DB 基盤 (core/)
      ├─ #Y 【実装】{機能}: API     (core/)
      ├─ #Z 【実装】{機能}: 画面    (examples/...)
      └─ ...
  → 親 Issue #N の本文に実装 Issue 一覧を task list で追記
  → 各 PR は対応 Issue を Closes
  → 全実装 Issue が close されると親 Issue の進捗バーが完了
  → 親 Issue を手動 Close (要求の最終確認後)
```

## リンク記法

モノレポなので `#番号` だけで十分:

```markdown
## 元 Issue

#42
```

## ラベル運用

| ラベル                                                                                | 意味                               | 付与タイミング                        |
| ------------------------------------------------------------------------------------- | ---------------------------------- | ------------------------------------- |
| `種別:バグ` / `種別:機能追加` / `種別:改善` / `種別:調査` / `種別:設計` / `種別:実装` | Issue 種別                         | 起票時                                |
| `状態:要求分析中`                                                                     | 要求文書を執筆・整理中             | 要求 Issue 起票時 (自動)              |
| `状態:設計着手OK`                                                                     | 要求が確定し、設計フェーズに入れる | `/requirements-publish` 通過時 (自動) |
| `状態:要求差し戻し`                                                                   | 設計担当が受け取り拒否             | `/spec-pickup` で差し戻し時           |
| `対象:基盤` / `対象:サーバー` / `対象:画面`                                           | 影響範囲                           | 起票時 (任意)                         |
| `優先:高` / `優先:中` / `優先:低`                                                     | 優先度                             | トリアージ時                          |

## 必須リンク

実装タスク Issue (`【実装】` プレフィックス) は本文に親要求 Issue へのリンクを必ず含める:

```markdown
## 元 Issue

#{親要求 Issue 番号}
```

## マイルストーン紐付け (必須)

**全ての要求 Issue / 実装 Issue は GitHub Milestone に紐付ける。**

理由: id-core は Phase 0〜7 の細粒度マイルストーン (M0.1, M0.2, ..., M7.2) で進捗管理を行う。
紐付けがないと、各 Issue がどのマイルストーンに属するか追跡できなくなる。

### 起票時の手順

1. 既存マイルストーン一覧を取得して、対象を特定する:

   ```bash
   gh api repos/{owner}/{repo}/milestones --jq '.[] | "\(.number) \(.title)"'
   # 例: 1 M0.1: core/ の最小 HTTP サーバー
   ```

2. `gh issue create` に `--milestone "{タイトル}"` または `--milestone {番号}` を付与する:

   ```bash
   gh issue create \
     --title "..." \
     --body "..." \
     --milestone "M0.1: core/ の最小 HTTP サーバー" \
     --label "種別:機能追加" --label "状態:要求分析中"
   ```

3. **どのマイルストーンに属するか不明な場合はユーザーに確認する** (推測で紐付けない)。

4. **対応するマイルストーンが存在しない場合**:
   - 既存マイルストーンの粒度では収まらない要求であれば、ユーザーに新規マイルストーン作成を提案する
   - スキルが勝手にマイルストーンを新設しない

### `/issue-from-spec` での扱い

設計書から実装 Issue を分割起票する場合、**全ての子 Issue を親要求 Issue と同じマイルストーンに紐付ける**。
要求 Issue がマイルストーンに紐付いていない場合は、その時点で起票を停止し、ユーザーに確認する。

### マイルストーン体系の参照

id-core の現行マイルストーン体系は以下を参照:

- GitHub UI: https://github.com/mktkhr/id-core/milestones
- API: `gh api repos/mktkhr/id-core/milestones`

## CLI

GitHub CLI (`gh`) を使用する。`glab` 系コマンドは利用しない。

```bash
# Issue 取得
gh issue view {番号}

# Issue 起票 (マイルストーン紐付け必須)
gh issue create --title "{タイトル}" --body "{本文}" --milestone "{マイルストーンタイトル}" --label "種別:..."

# マイルストーン一覧
gh api repos/{owner}/{repo}/milestones --jq '.[] | "\(.number) \(.title)"'

# Issue 更新 (本文・ラベル)
gh issue edit {番号} --body "{新本文}" --add-label "..." --remove-label "..."

# コメント
gh issue comment {番号} --body "{本文}"
```

## 自動 Close

モノレポなので PR 本文に `Closes #N` を書けば自動 Close される (クロスプロジェクトの制約なし)。
親要求 Issue (`docs/requirements/{N}/`) は実装完了後、ユーザーが手動で Close する。

## PR 運用ポリシー

PR の作成・レビュー・マージは `.rulesync/rules/pr-review-policy.md` に集約済み。要点:

- **Codex レビュー必須** (`/pr-codex-review {番号}`)。ゲート: CRITICAL=0 / HIGH=0 / MEDIUM<3
- **assignee + labels (種別) 必須**
- **バージョン変更 (go.mod / package.json / Action uses 等) は事前承認必須**
- **PR description にアーカイブ参照や固有事情を書かない**
- **裸の `@<word>` 表記 (誤メンション源) 禁止**

詳細は `pr-review-policy.md` を参照。
