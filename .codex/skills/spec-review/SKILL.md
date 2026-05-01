---
name: spec-review
description: >-
  設計書を Codex にレビュー委譲する。"設計書レビュー", "Codex にレビュー", "レビューしてもらって", "spec-review"
  等で発動。
---
# 設計書レビュースキル

設計書 (`docs/specs/{N}/index.md`) を Codex に委譲してレビューする。

詳細ルールは `.rulesync/rules/codex-delegation.md` および
`.rulesync/rules/path-resolution.md` に従う。

## 入力

- `$ARGUMENTS` から対象の設計書パスを特定する
  - パス指定: そのファイルを対象とする
  - 引数なし: 会話コンテキストから対象設計書を推定する

## レビュー実行手順

### 1. 背景情報の収集

レビュー精度を上げるため、以下を自動で収集してプロンプトに含める。
パスは全てリポジトリルートからの相対パスで解決する。

| 観点                    | 参照先                                                              |
| ----------------------- | ------------------------------------------------------------------- |
| 対象設計書              | `docs/specs/{N}/index.md`                                           |
| 既存マイグレーション    | `core/db/migrations/*.up.sql` (存在すれば)                          |
| 既存 OpenAPI            | `core/api/openapi.yaml`, `core/api/components/` (存在すれば)        |
| 既存エラー定義          | `core/internal/apperror/*.go` (存在すれば)                          |
| **認可マスター (正本)** | `docs/context/authorization/matrix.md`                              |
| アーキテクチャ          | `docs/context/app/architecture.md`                                  |
| backend 規約            | `docs/context/backend/conventions.md`, `patterns.md`, `registry.md` |
| 関連画面仕様            | 設計書の関連資料セクションから特定                                  |

### 2. Codex 委譲

```bash
codex exec --full-auto \
  "You are reviewing a detailed design document at docs/specs/{N}/index.md.

   Read the following context files first:
   - docs/specs/{N}/index.md
   - docs/context/app/architecture.md
   - docs/context/authorization/matrix.md
   - docs/context/backend/{conventions,patterns,registry}.md
   - core/db/migrations/*.up.sql (if exists)
   - core/api/openapi.yaml (if exists)

   Review focus:
   1) ER 図 / DDL の整合性 (型、制約、命名、NULL 可否)
   2) ドメインロジック / トランザクション境界の漏れ
   3) API 設計の妥当性 (エンドポイント、認可、エラーコード)
   4) **認可マスターとの一致** (docs/context/authorization/matrix.md と設計書の認可表が 1 セルでも食い違わないこと)
   5) OIDC OP としての仕様適合 (RFC 6749 / OIDC Core / 必要に応じて RFC 8628 等)
   6) 既存コード規約との一貫性
   7) リクエスト / レスポンス JSON 構造
   8) 未確定事項の追加提案
   9) その他気づいた問題点

   Be specific: cite table names, column names, file:line.
   Output as structured Markdown in Japanese.
   At the end, include '## Summary' with severity counts (CRITICAL/HIGH/MEDIUM/LOW)."
```

### 3. 結果の保存と報告

| 成果物       | パス                                    |
| ------------ | --------------------------------------- |
| レビュー全文 | `.ai-out/YYYY-MM-DD-spec-{N}-review.md` |

ユーザーへは重大度別の要約 (Summary セクション) のみ読み取り、報告する。
全文の保存先パスを併記する。

## 注意

- Codex の出力全文は読み込まない (コンテキスト保護)
- 認可マスター差分が検出された場合は `.rulesync/rules/authorization-matrix.md` に従い停止する
