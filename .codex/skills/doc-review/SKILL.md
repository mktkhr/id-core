---
name: doc-review
description: >-
  設計書・要求文書を Claude Code でセルフレビューする。"セルフレビュー", "ローカルレビュー", "doc-review",
  "自分でレビュー", "ドキュメントレビュー" 等で発動。
---
# 設計書・要求文書セルフレビュー

設計書 (`docs/specs/` 配下) や要求文書 (`docs/requirements/` 配下) を Claude Code 単体でレビューする。

## 前提として読み込む context

**必須** (必ず読む):
- レビュー対象のドキュメント

**条件付き** (レビュー対象に含まれる領域に応じて該当時のみ読む):
- DB/API/エラー設計を含む: `docs/context/backend/{conventions,patterns,registry}.md` のうち**該当する 1〜2 ファイルのみ**
- 画面設計を含む: `docs/context/frontend/{conventions,patterns,registry}.md` のうち**該当する 1〜2 ファイルのみ**
- テスト観点を含む: `docs/context/testing/{backend,e2e}.md` のうち**該当する 1 ファイルのみ**
- 認可記述あり: `docs/context/authorization/matrix.md`
- プロジェクト全体整合をレビューする場合のみ: `docs/context/app/architecture.md`

**読み込まない**: レビュー対象の領域に**関係ない context** は読まない。「念のため全部読む」は禁止。

詳細は `docs/context/README.md` の対応表を参照。

## 使い方

```
/doc-review                                  # 会話コンテキストから対象を推定
/doc-review docs/specs/1/index.md            # 指定ファイル
/doc-review docs/requirements/1/index.md     # 要求文書も対象
```

## 手順

1. `$ARGUMENTS` から対象のドキュメントパスを特定する
   - パス指定: そのファイルを対象とする
   - 引数なし: 会話コンテキストから対象を推定する

2. 対象ドキュメントを読み込む

3. 必要に応じてサブエージェントを活用した包括的な分析を行う:
   - **DB 整合性 sub-agent**: ER 図・DDL の型、制約、命名、NULL 可否の整合
   - **API 整合性 sub-agent**: エンドポイント、リクエスト / レスポンス JSON、エラーコード
   - **ドメインロジック sub-agent**: トランザクション境界、バリデーション、ビジネスルール
   - **認可整合性 sub-agent**: 認可マトリクス (`docs/context/authorization/matrix.md`) との突合

## レビュー観点

### 内部整合性 (CRITICAL)

- ER 図と DDL の不一致 (型、制約、カラム名)
- API エンドポイント定義と JSON 構造の矛盾
- 認可マトリクスとロール別 UI 制御の不一致
- エラーコードの重複・欠落

### 規約準拠 (HIGH)

- テーブル名・カラム名の命名規則 (snake_case)
- API パスの命名規則 (ケバブケース、RESTful、OIDC 標準準拠)
- エラーコードのフォーマット
- data-testid 命名規則 (`{画面}-{要素種別}-{名前}`)

### 完成度 (HIGH)

- TODO / TBD / 未定 が残っていないか
- 全 API エンドポイントにリクエスト / レスポンス定義があるか
- 全テーブルに DDL があるか
- シーケンス図・フローチャートが全ドメイン操作をカバーしているか

### 設計品質 (MEDIUM)

- N+1 クエリの懸念がある設計
- トランザクション境界が不明確
- 冗長な API コール設計
- 正規化 / 非正規化の判断理由が未記載

### 既存資料との整合 (MEDIUM)

- `docs/context/backend/registry.md` 等との一貫性
- 既存マイグレーションとの互換性
- 既存エラー定義との重複

## レポート生成

各問題について以下を含むレポートを出力:

- **重要度**: CRITICAL, HIGH, MEDIUM, LOW
- **セクション**: 問題のあるドキュメントセクション名
- **問題の説明**
- **修正案**

## 自動修正対象

以下のみ自動修正可:
- typo (誤字脱字)
- リンク切れ
- 見出し整形 (Markdown 見出しレベル・番号揃え)
- 表組の体裁 (カラム整形・空白調整)

これ以外は重大度に関係なく**ユーザー確認**が必要。
