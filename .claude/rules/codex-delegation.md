# Codex 委譲ルール

## 概要

Claude Code と Codex CLI の 2 者オーケストレーション。
**重い分析・横断調査・全文レビュー**は Codex に委譲し、Claude Code のコンテキストを保護する。

## Codex に委譲するタスクの目安

### 必ず委譲

1. **全文レビュー (設計書 / マイグレーション / OpenAPI)**
   - `core/` 配下の Go コード横断レビュー
   - `docs/specs/{N}/index.md` の整合性チェック
   - `docs/context/authorization/matrix.md` 全体スキャン
2. **複数ファイルにまたがる一貫性チェック**
   - エンドポイント一覧 ↔ 認可マトリクス ↔ 認可ポリシー実装
   - DDL ↔ ER 図 ↔ API レスポンス JSON
   - context/registry の宣言 ↔ 実装の存在確認
3. **横断的な調査**
   - 「どこで使われているか」を全文 grep + 解釈する系
   - 規約遵守度の網羅監査

### 委譲を推奨

- Claude Code の計画・設計判断に対するセカンドオピニオン
- 公開ドキュメントの Web 検索・リサーチ (コンテキスト保護のため)

## Plan mode ガード (必須)

- Codex 委譲コマンドの実行前に、現在が Plan mode かを確認する
- Plan mode の場合は、必ずユーザーに次を確認する:
  - `Plan mode を終了して Codex 委譲を実行しますか？ (Y/N)`
- ユーザーの明示承認があるまで、`codex exec` を実行しない
- 承認後に `ExitPlanMode` を実行してから委譲を開始する

## Claude Code が直接行うタスク

- 設計書 / 要求文書 / ADR の対話的執筆・編集
- マイグレーション / コード生成 (規模が小さいもの)
- GitHub Issue 本文の作成・起票・更新 (`gh` 経由)
- コミット作成
- ユーザーとの対話

## Codex 呼び出しパターン

呼び出しは `/ask-codex` スキル経由を推奨。

```bash
# 基本形: full-auto で非対話実行
codex exec --full-auto "プロンプト内容"

# 設計書レビュー (例)
codex exec --full-auto \
  "Review docs/specs/0001/index.md against core/db/migrations/*.up.sql and \
   docs/context/authorization/matrix.md. Check ER↔DDL consistency, \
   authorization-master alignment, and API contract drift. \
   Report issues as CRITICAL/HIGH/MEDIUM/LOW in Japanese."
```

## 結果の管理

- 保存先: `.ai-out/YYYY-MM-DD-{topic}.md`
- Codex の出力は **Summary セクションのみ** 読み取り、コンテキストに返却
- 全文は保存ファイルで参照可能

## 禁止事項

- Codex に**書き込み権限**を与えるタスクを委譲しない (読み取り・分析のみ)
- リポジトリ外の絶対パスを Codex プロンプトに含めない (`.rulesync/rules/path-resolution.md` 準拠)
- 結果ファイル全文を Claude Code のコンテキストに読み込まない
