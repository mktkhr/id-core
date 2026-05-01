---
name: issue-triage
description: >-
  未整理の Issue にラベル・優先度を提案する。"issue を整理", "未整理の issue", "issue-triage", "ラベル付けして"
  等で発動。
---
# Issue 整理スキル

未ラベルの Issue を一括で読み取り、ラベル・優先度を提案する。

## 手順

### 1. 未整理 Issue の取得

`gh issue list` でオープン Issue を一覧取得し、ラベルが不足しているものを抽出する。
「不足」の判断基準: **種別ラベル**または**優先度ラベル**がない。

```bash
gh issue list --state open --json number,title,labels --limit 100
```

### 2. 提案の作成

各 Issue について以下を提案する:

```
| Issue | 現在のラベル | 追加提案 | 理由 |
|---|---|---|---|
| #1 タイトル | (なし) | 種別:バグ, 優先:中, 対象:画面 | 再現手順が記載されておりバグ報告 |
| #2 タイトル | 種別:機能追加 | 優先:低 | スコープが小さく緊急性なし |
```

### 3. ユーザー承認後に適用

承認されたラベルのみ `gh issue edit` で付与する。

```bash
gh issue edit {番号} --add-label "種別:バグ" --add-label "優先:中"

# 古いラベルを外す場合
gh issue edit {番号} --remove-label "優先:低"
```

## CLI リファレンス

```bash
# Issue 一覧 (オープン)
gh issue list --state open

# JSON で詳細
gh issue list --json number,title,labels,state --limit 100

# ラベル付与・削除
gh issue edit {番号} --add-label "..." --remove-label "..."
```

## 注意

- 同カテゴリ (例: `優先:`) のラベルが既にある場合、古いものを `--remove-label` で外す
- 判断に迷うものは「保留」として報告し、ユーザーに委ねる
- リポジトリにラベルが未作成なら、初回のみ `gh label create` で作っておく
