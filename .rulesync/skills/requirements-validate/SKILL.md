---
name: requirements-validate
description: '要求文書の完成度を設計着手条件で検証する。"requirements-validate", "設計着手条件チェック" 等で発動。'
targets:
  - "*"
---

# 要求着手条件検証スキル

要求文書が設計工程へ引き渡せる状態かを検証する。

## 前提として読み込む context

**必須** (必ず読む):
- 対象の `docs/requirements/{N}/index.md`

**条件付き** (該当する場合のみ読む):
- 要求に**認可記述あり**: `docs/context/authorization/matrix.md` (突合済かの確認)

**読み込まない**: 上記以外の context は読まない。

詳細は `docs/context/README.md` の対応表を参照。

## 標準の設計着手条件

- 業務ルール
- 入出力要件
- 例外シナリオ
- バリデーション方針
- 権限要件 (記述がある場合はマスター突合済)
- 監査 / 非機能
- 未決事項ゼロ、または決定者 / 期限付き

## 手順

1. `docs/requirements/*/index.md` の必須セクション埋まり具合を確認
2. 未決事項テーブルを確認
3. 認可記述がある場合はマスター (`docs/context/authorization/matrix.md`) と突合済みか確認
4. NG 項目を一覧化して修正提案
5. 全て OK なら `設計着手可` を記録

## 運用

- 1 件でも NG があれば停止する (警告で進めない)
- 進行可否は `合格 / 不合格` の二値で返す
