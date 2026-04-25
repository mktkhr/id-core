---
name: requirements-create
description: '要求の初期ドラフトを生成する。"requirements-create", "要求ドラフト", "要件の下書き", "新規要求" 等で発動。'
---
# 要求ドラフト作成スキル

要求文書の初期版を `docs/templates/requirements/template.md` から生成する。

## 前提として読み込む context

**必須** (必ず読む):
- `docs/templates/requirements/template.md` — 要求文書テンプレート

**条件付き** (該当する場合のみ読む):
- 要求の入力に**認可記述あり**: `docs/context/authorization/matrix.md` (突合必須・推測禁止)
- 初回起動 / プロジェクトのスコープが把握できていない: `docs/context/app/architecture.md` (要求がスコープ内かの判定)

**読み込まない**: backend/frontend/testing 配下の詳細 context は要求段階では不要。

詳細は `docs/context/README.md` の対応表を参照。

## 入力

- `$ARGUMENTS` が数字のみ: 関連 Issue 番号として扱う
- `$ARGUMENTS` が自由文: 要求の種として扱う
- 引数なし: 会話コンテキストから推定する

## 手順

1. 入力種別を判定する
   - Issue 番号起点: Issue 内容を取得 (リポジトリで Issue tracker を運用していれば確認、なければユーザーに概要を聞く)
   - 自由文起点: 背景 / 目的 / 対象ユーザーを聞き取り、最小セットを埋める

2. 出力先を決める
   - Issue 番号起点: `docs/requirements/{issue_id}/index.md`
   - 自由文起点: `docs/requirements/{連番 or YYYYMMDD-slug}/index.md`
   - 連番方式の場合、`ls docs/requirements/ | sort -V` で最大番号を確認して `+1`

3. **既存ドラフトの検出**
   - 出力先に `index.md` が既に存在する場合は上書きせず、
     「既存ドラフトが見つかりました。続行するか上書きするか」をユーザーに確認する

4. `docs/templates/requirements/template.md` をコピーして初期値を埋める

5. 「要求フェーズ状況」のフェーズ 1 (ドラフト) を `完了`、以降を `未着手` に更新する

6. 次は `/requirements-validate` で不足を検出するよう提案する

## 注意

- DDL / ER / API 詳細は記載しない (それらは設計書の責務)
- 認可記述が登場した場合は `docs/context/authorization/matrix.md` と突合する
  (詳細は `.rulesync/rules/authorization-matrix.md`)

## 関連スキル

- `/requirements-resolve` — 未決事項を 1 件ずつ解決
- `/requirements-review` — レビュー
- `/requirements-validate` — 設計着手条件を満たすか検証
- `/requirements-track` — 変更履歴の追記
- `/requirements-full` — 全フェーズを通しで実行
