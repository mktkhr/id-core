---
name: requirements-resolve
description: '要求文書の未決事項を1件ずつ解決して反映する。"requirements-resolve", "未決解消", "要求の論点整理" 等で発動。'
---
# 要求論点解決スキル

`docs/requirements/*/index.md` の「未決事項 (論点)」から 1 件を解決し、要求本文に反映する。

## 前提として読み込む context

**必須** (必ず読む):

- 対象の `docs/requirements/{N}/index.md` (該当論点セクションのみで足りる場合は部分読み可)

**条件付き** (該当する場合のみ読む):

- 論点が**認可関連**: `docs/context/authorization/matrix.md`

**読み込まない**: 論点に関係ない context は読まない。

詳細は `docs/context/README.md` の対応表を参照。

## 入力

- `$ARGUMENTS` で論点番号または論点名を指定
- 引数なし: 未決一覧を表示して選択を促す

## 手順

1. 対象論点の背景・選択肢・影響範囲を整理する
2. ユーザーの決定を確認する
3. 決定内容を次へ反映する
   - 業務ルール
   - 入出力要件
   - 例外シナリオ
   - バリデーション方針
   - 権限要件 (該当時)
4. 未決事項テーブルの状態を更新する
5. 変更を「変更履歴」に追記する (重複は追加しない)

## 停止条件

- 認可に関わる論点はマスター (`docs/context/authorization/matrix.md`) との突合完了前に確定しない
  (詳細は `.rulesync/rules/authorization-matrix.md`)
- 決定者 / 期限が決められない場合は `保留` のまま停止する
