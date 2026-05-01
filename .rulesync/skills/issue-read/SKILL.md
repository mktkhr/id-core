---
name: issue-read
description: >-
  GitHub Issue を読み取り要約する。"issue 確認", "issue を読んで", "issue-read", "#番号の内容",
  Issue 番号の言及で発動。
targets:
  - "*"
---

# Issue 読み取りスキル

GitHub Issue を読み取り、要約して返す。

## 入力

- `$ARGUMENTS` から対象 Issue を特定する
  - 数字のみ: Issue 番号として扱う
  - URL: Issue 番号を抽出する
  - 引数なし: ユーザーに Issue 番号を確認する

## 手順

1. `gh issue view {番号}` で Issue を取得する
2. 必要ならコメントも取得: `gh issue view {番号} --comments`
3. 以下の形式で要約を返す:

```
## Issue #{番号}: {タイトル}

| 項目 | 値 |
|---|---|
| 状態 | open / closed |
| ラベル | ... |
| 担当者 | ... |
| マイルストーン | ... |
| 作成日 | ... |
| 更新日 | ... |

### 概要
（本文の要約 5-8 行）

### 次にやること
（推奨アクション。例: `/spec-pickup {番号}`、`/issue-from-spec docs/specs/{N}/`）
```

## CLI リファレンス

```bash
# Issue 詳細
gh issue view {番号}

# コメントも含めて取得
gh issue view {番号} --comments

# Issue 一覧
gh issue list
```
