---
root: false
targets:
  - "*"
---

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

## CLI

GitHub CLI (`gh`) を使用する。`glab` 系コマンドは利用しない。

```bash
# Issue 取得
gh issue view {番号}

# Issue 起票
gh issue create --title "{タイトル}" --body "{本文}" --label "種別:..."

# Issue 更新 (本文・ラベル)
gh issue edit {番号} --body "{新本文}" --add-label "..." --remove-label "..."

# コメント
gh issue comment {番号} --body "{本文}"
```

## 自動 Close

モノレポなので PR 本文に `Closes #N` を書けば自動 Close される (クロスプロジェクトの制約なし)。
親要求 Issue (`docs/requirements/{N}/`) は実装完了後、ユーザーが手動で Close する。
