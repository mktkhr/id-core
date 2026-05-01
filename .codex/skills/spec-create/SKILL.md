---
name: spec-create
description: '要求から設計書の雛形を生成する。"設計書作成", "spec-create", "雛形生成", "設計書を作って" 等で発動。'
---
# 設計書作成スキル

要求文書または対話入力をもとに、テンプレートから設計書の雛形を生成する。

## 前提として読み込む context

**必須** (必ず読む):

- `docs/templates/specs/template.md` — 雛形生成の元
- 該当する `docs/requirements/{N}/index.md` — 要求起点の場合

**条件付き** (該当する場合のみ読む):

- 要求 / 対象機能に**認可記述あり**: `docs/context/authorization/matrix.md` (突合必須・推測禁止)
- 初回起動 / プロジェクト全体像が把握できていない: `docs/context/app/architecture.md`

**読み込まない**: backend/frontend/testing 配下の詳細 context は spec-create 段階では不要。雛形生成後、`/check-convention` や `/spec-resolve` が必要な context を選択的に読む。

詳細は `docs/context/README.md` の対応表を参照。

## 入力

- `$ARGUMENTS`: 要求番号 / Issue 番号 / 機能名
- 引数なし: ユーザーに対象を確認する

## 手順

### 1. 要求情報の取得

- 関連する要求文書 (`docs/requirements/{N}/index.md`) を読み取る
- なければユーザーに要件をヒアリング

### 2. ディレクトリ作成

```
docs/specs/{番号 or 機能名}/
├── index.md                # 設計書本体
└── prompts/                # 実装プロンプト (後で生成)
```

- ディレクトリ名はケバブケースまたは番号ベース
- 既に存在する場合は上書きせず、差分のみ提案する

### 3. テンプレートから雛形を生成

`docs/templates/specs/template.md` をベースに、以下を埋める:

- **タイトル**: 要求の件名から
- **関連資料**: 要求文書 / 認可マトリクス / 関連 ADR へのリンク
- **要件の解釈**: 要求の内容を構造化して記載
- **実装対象**: id-core / go-react / kotlin-nextjs / DB のどこに変更が入るか判定
- **設計時の論点**: 要件から読み取れる未確定事項を論点テーブルに記載

### 3-1. 認可マスターとの突合 (必須)

**要求 / 対象機能のいずれかに認可記述がある場合は必ずマスター正本と突合する。**
権限に関する記述が一切ない場合のみ本手順を省略可。
詳細は `.rulesync/rules/authorization-matrix.md` を参照。

1. `docs/context/authorization/matrix.md` (認可マスター正本) から対象機能の行を抽出
2. 要求文書の権限欄と突合
3. **1 セルでも差分があれば停止**してユーザーに提示
4. 差分ゼロを確認後、設計書の認可マトリクスはマスターの値を**そのまま**コピー (独自判断で拡大 / 縮小しない)

### 4. 初期論点の洗い出し

要求文書から明確でない点を論点テーブルに列挙:

- 振る舞い (条件分岐、エラー表示等)
- DB 設計 (テーブル構成、カラム型、制約)
- API 設計 (エンドポイント粒度、認可方式、OIDC 標準への準拠)
- 既存機能への影響

## 生成後の提案

雛形生成後、設計ワークフロー (`.rulesync/rules/design-workflow.md`) に従い次のステップを提案:

- 論点がある場合 → `/spec-resolve` で論点を 1 つずつ解決
- 規約確認が必要 → `/check-convention` で既存規約を確認
- 全体を通しで進めたい → `/spec-full`
