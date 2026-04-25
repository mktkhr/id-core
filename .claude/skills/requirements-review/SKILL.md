---
name: requirements-review
description: '要求文書をレビューして重大な不足や矛盾を検出する。"requirements-review", "要求レビュー" 等で発動。'
---
# 要求レビュー スキル

要求文書をレビューし、重大度別に指摘を返す。

## 前提として読み込む context

**必須** (必ず読む):
- 対象の `docs/requirements/{N}/index.md`

**条件付き** (該当する場合のみ読む):
- 要求に**認可記述あり**: `docs/context/authorization/matrix.md` (突合のため)

**読み込まない**: 上記以外の context は読まない。

詳細は `docs/context/README.md` の対応表を参照。

## レビュー観点

1. 目的 / 価値と業務ルールの整合
2. 入出力 / 例外 / バリデーションの整合
3. 権限要件とマスター認可 (`docs/context/authorization/matrix.md`) の整合
4. 未決事項と設計着手条件判定の整合
5. スコープ (やること / やらないこと) の明確性
6. 非機能要件 (パフォーマンス・セキュリティ・可用性) の最低ラインの記載

## 出力

- CRITICAL / HIGH / MEDIUM / LOW で分類して報告
- CRITICAL / HIGH は自動修正しない
- 軽微修正 (誤字、表整形、見出し整形) のみ自動修正対象

## 運用

- フェーズ 2 以降で必ず実行する
- CRITICAL / HIGH が 1 件でもあれば次フェーズへ進めない
