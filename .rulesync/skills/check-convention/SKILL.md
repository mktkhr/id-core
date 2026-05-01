---
name: check-convention
description: >-
  既存コードから規約を調査する。"規約確認", "既存の形式を確認", "check-convention", "マイグレーションの形式",
  "エラーの形式", "API の命名規則" 等で発動。
targets:
  - "*"
---

# 規約確認スキル

設計の前に、既存コードや context/ から規約・パターンを調査して報告する。
このスキルは context/ 全般を横断的に読み取る性質があるため、必読 context は本ファイル下記の「実行手順 1」内表で詳細指定する。
全体マッピングは `docs/context/README.md` の「skill → 必読 context 対応表」を参照。

## 入力

- `$ARGUMENTS` から調査対象の観点を特定する
  - 例: "マイグレーション形式", "エラーレスポンス", "API 命名規則", "FK 方針", "認証セッション管理"
  - 引数なし: context/ の概要を報告する

## 実行手順

### 1. context/ を読む (必須・最初に実行)

`docs/context/` 配下の該当ファイルを読み、既存規約の概要を把握する:

| 観点                                                                      | 参照先                                                                                    |
| ------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| バックエンド全般                                                          | `docs/context/backend/conventions.md`, `docs/context/backend/patterns.md`                 |
| バックエンドの実体一覧 (テーブル / API / エラーコード / マイグレーション) | `docs/context/backend/registry.md`                                                        |
| フロントエンド全般                                                        | `docs/context/frontend/conventions.md`, `docs/context/frontend/patterns.md`               |
| フロントエンドの画面 / data-testid 一覧                                   | `docs/context/frontend/registry.md`                                                       |
| 認可                                                                      | `docs/context/authorization/matrix.md` (正本) + `.rulesync/rules/authorization-matrix.md` |
| アプリ全体構成                                                            | `docs/context/app/architecture.md`                                                        |
| テスト                                                                    | `docs/context/testing/backend.md`, `docs/context/testing/e2e.md`                          |

**大半のケースはこれだけで回答できる。ソースコードは読まない。**

### 2. context で不足する場合のみソースを確認

以下に該当する場合のみ、最小限のソースを読みに行く:

- context に記載がない新しい観点
- context の記載が古い可能性がある (マイグレーション番号が増えている等)
- 特定のファイルの具体的な実装パターンが必要

#### ソース確認が必要な場合の対象

| 観点                   | 確認先                                                                              |
| ---------------------- | ----------------------------------------------------------------------------------- |
| DB / マイグレーション  | `core/db/migrations/*.up.sql`                                                       |
| API                    | `core/api/openapi.yaml`                                                             |
| エラー定義             | `core/internal/apperror/` (実装後)                                                  |
| 認可ポリシー           | `docs/context/authorization/matrix.md` (正本) + `core/db/migrations/*authz*.up.sql` |
| OIDC OP エンドポイント | `core/internal/features/oidc/` (実装後)                                             |

**注意**: ソースを読む場合も、関連するファイルだけをピンポイントで読む。全ファイルを一括で読まない。

### 3. context の更新

ソースから新しい規約を発見した場合、`docs/context/` の該当ファイルに追記する。
次回以降の調査でソースを読む必要がなくなるようにする。

### 4. 報告

調査結果を簡潔に報告する。設計書への反映は別途ユーザーの指示を待つ。
