---
name: requirements-full
description: '要求定義を端から端まで実行する。"requirements-full", "要求を固める", "要件前段" 等で発動。'
targets:
  - "*"
---

# 要求フル実行スキル

要求定義を、停止ゲート付きで段階実行する。

## 前提として読み込む context

オーケストレータ。**直接の必読 context は持たない** (各サブスキルが必要に応じて読む)。

各フェーズで起動するサブスキル (`/requirements-create`, `/requirements-resolve`, `/requirements-review`, `/requirements-validate`, `/requirements-track`) が、それぞれの「前提として読み込む context」セクションに従って **必要な context だけを** 選択的に読む。

**読み込まない**: requirements-full 自体が context を一括ロードしてはならない。token 浪費の温床になる。

詳細は `docs/context/README.md` の対応表を参照。

## 入力

- `$ARGUMENTS`: 要求番号 / Issue 番号 / 自由文
- 引数なし: 会話コンテキストから推定する

## 中核原則

- **未決事項が 1 件でも残る場合は次フェーズへ進まない**
- フェーズ 2 以降は毎フェーズ `/requirements-review` を実行する
- CRITICAL / HIGH は自動修正しない
- 認可記述が出た時点でマスター (`docs/context/authorization/matrix.md`) との突合を必ず行う
- ユーザー応答待ちタイムアウトは禁止 (「反応がないので進めます」は禁句)

## 完了ゲート (フェーズ 9 実行条件・厳格)

要求文書を「設計着手 OK」にするには以下を**全て**満たす必要がある:

| 項目                           | 閾値                      |
| ------------------------------ | ------------------------- |
| `requirements-validate`        | 合格                      |
| `requirements-review` CRITICAL | 0 件                      |
| `requirements-review` HIGH     | 0 件                      |
| `requirements-review` MEDIUM   | 3 件未満 (0/1/2 件のみ可) |

**未達なら次工程に進まない。** ユーザーが「そのまま進めて」と言ってもゲートは緩めない。

## フェーズ

| #   | フェーズ            | 対応スキル                                        | 停止条件                                 |
| --- | ------------------- | ------------------------------------------------- | ---------------------------------------- |
| 1   | 受付                | `/requirements-create`                            | 入力起点が不明                           |
| 2   | 下書き              | `/requirements-create`                            | 必須セクション不足                       |
| 3   | スコープ / 成功条件 | 手動更新                                          | In/Out 不明                              |
| 4   | 業務仕様            | 手動更新                                          | 業務ルールに曖昧さ                       |
| 5   | バリデーション方針  | 手動更新                                          | 例外時の扱い不明                         |
| 6   | 権限要件            | マスター突合                                      | マスター差分                             |
| 7   | 監査 / 非機能       | 手動更新                                          | 最低要件未定                             |
| 8   | 未決事項解消        | `/requirements-resolve`                           | 未決が残存                               |
| 9   | 最終レビュー        | `/requirements-validate` + `/requirements-review` | 設計着手条件を満たさない or 重大指摘あり |
| 10  | 差分整理            | `/requirements-track`                             | 変更履歴の追記漏れ                       |

## 完了条件

- フェーズ 10 まで完了
- 要求文書に Issue 情報 (あれば) が記載済み
- 設計着手条件を満たしている (validate 合格)
- CRITICAL / HIGH ゼロ / MEDIUM 3 件未満 (公開ゲート通過)
- 設計工程へ引き継ぎ可能な状態

## 完了時の案内

完了後、以下を必ずユーザーに報告:

```
🎉 要求定義が完了しました。

【要求文書】 docs/requirements/{N}/index.md
【状態】 設計着手 OK

次工程:
  /spec-create {N} で設計書の雛形を作成し、設計フェーズに移行します。
```

## 禁止事項

- 未決を残したままフェーズ 9 に進む
- レビュー指摘 (CRITICAL/HIGH) を独断で修正
- ユーザー確認なしの次フェーズ移行
- ユーザー応答待ちタイムアウト
