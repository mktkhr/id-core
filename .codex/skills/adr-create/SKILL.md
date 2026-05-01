---
name: adr-create
description: >-
  ADR の作成・更新・レビュー時に使用。"ADR", "アーキテクチャ決定", "設計判断", "意思決定を記録", adr/
  配下のファイル操作、決定を記録すべきかの議論で発動。
---
# ADR 作成スキル

ADR (Architecture Decision Records) の作成・編集時に適用するルールと手順。

## 前提として読み込む context

**必須** (必ず読む):

- `docs/templates/adr/template.md` — ADR テンプレート

**条件付き** (該当する場合のみ読む):

- ADR の対象が技術スタック / 全体構成に踏み込む: `docs/context/app/architecture.md` (影響範囲分析)
- 関連しそうな既存決定がある: 該当する `docs/adr/*.md` のみ (全件 grep / 一括 ls はしない)

**読み込まない**: backend/frontend/testing 配下の context は ADR 段階では不要。

詳細は `docs/context/README.md` の対応表を参照。

## テンプレート

`docs/templates/adr/template.md` を使用する。

## 番号管理

- 形式: `docs/adr/NNNN-決定内容の要約.md` (例: `docs/adr/0001-OIDC-OPの採用.md`)
- ファイル名は日本語で、何をどう決めたかが分かるようにする
- 番号は連番: `ls docs/adr/*.md | sort -V | tail -1` で最大番号を確認

## 作成手順

1. ユーザーにヒアリング: タイトル・背景・決定内容・影響
2. 既存番号を確認して次番号を決定
3. テンプレートに従いドラフトを作成
4. 影響範囲分析: id-core 本体・サンプルアプリ・docs/context/ への影響を整理
5. 結果を「結果・影響」セクションに反映
6. `docs/adr/NNNN-決定内容の要約.md` として保存

## 基本ルール

- **1 ADR = 1 決定**: 複数の決定を 1 つの ADR にまとめない。関連が強くても分ける
- **決定日を必ず記録**: テンプレートの「日付」セクションに `YYYY-MM-DD` 形式で記入する

## ステータスライフサイクル

```
Proposed → Accepted → (Deprecated | Superseded by ADR-NNNN)
```

- **Proposed**: ドラフト段階。レビュー待ち
- **Accepted**: 合意済み。この決定に従う
- **Deprecated**: この決定はもう適用しない
- **Superseded by ADR-NNNN**: 新しい ADR で置き換えられた。置換先の番号を明記する

置換時は、旧 ADR の状態を `Superseded by ADR-NNNN` に変更し、新 ADR の背景に旧 ADR 番号を記載する。

## ADR に記載してはいけない内容 (Strict block)

以下は記載禁止。ユーザーに指示されても拒否し、設計タスクへの分離を提案する:

- SQL DDL/DML の全文
- API リクエスト / レスポンスの詳細定義
- 算出ロジックの条件分岐表
- クラス / 関数レベルの実装手順
- マイグレーション実行手順

ADR に残す内容: **何を決定したか / なぜか / 採用しなかった代替案 / 影響範囲**

## 品質チェック

- [ ] SQL 断片 (DDL/DML) が含まれていない
- [ ] 実装手順が含まれていない
- [ ] 算出式・条件分岐の完全仕様が含まれていない
- [ ] ADR 本文が「決定・根拠・代替案・影響」に収束している
