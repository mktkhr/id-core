---
name: requirements-publish
description: >-
  要求確定後に GitHub Issue へ反映し公開する。"requirements-publish", "要求を Issue 化", "要求を同期"
  等で発動。
---
# 要求公開スキル

要求文書を確定し、GitHub Issue へ同期する。
公開ゲートを満たさない限り、Issue への反映・ラベル付与は行わない。

## 前提

- `/requirements-validate` を実行済み (合格必須)
- `/requirements-review` を実行済み

## 公開ゲート (必須・厳格)

以下を**全て**満たした場合のみ公開可:

| 項目                           | 閾値                      |
| ------------------------------ | ------------------------- |
| `requirements-validate`        | 合格                      |
| `requirements-review` CRITICAL | 0 件                      |
| `requirements-review` HIGH     | 0 件                      |
| `requirements-review` MEDIUM   | 3 件未満 (0/1/2 件のみ可) |

**1 項目でも未達なら公開を拒否し、未達項目と該当セクションを提示して停止する。**
ユーザーが「そのまま公開して」と言っても、ゲート通過なしには公開しない。

## 手順

### 1. ゲート判定

1. `/requirements-validate` を内部実行 → NG なら停止
2. `/requirements-review` を内部実行 → CRITICAL/HIGH/MEDIUM 件数を取得
3. 閾値と照合
   - 未達: 該当項目と修正対象セクションを一覧化して停止
   - 通過: 次ステップへ

### 2. Issue への同期

1. 要求文書 (`docs/requirements/{N}/index.md`) から対応 Issue 番号を確認
   - ヘッダーや「関連 Issue」セクションに番号があるか
2. Issue の状態に応じて分岐:
   - 既存 Issue あり: `gh issue edit {番号} --body "{新本文}"` で同期
   - Issue なし: ユーザーに確認して `gh issue create` で新規作成
3. 作成/更新後、Issue 番号・URL を要求文書に反映する

### 3. ラベル付与 (必須)

公開成功時、以下を**必ず**実行する:

```bash
gh issue edit {番号} \
  --add-label "状態:設計着手OK" \
  --remove-label "状態:要求分析中" \
  --remove-label "状態:要求差し戻し"
```

(該当ラベルがない場合は `--remove-label` のエラーは無視してよい)

### 4. 公開記録

要求文書 (`docs/requirements/{N}/index.md`) に以下を追記:

```markdown
## 公開記録

- 公開日時: {YYYY-MM-DD HH:MM}
- Issue: {Issue URL}
- ゲート結果:
  - validate: 合格
  - CRITICAL: 0 / HIGH: 0 / MEDIUM: {件数}
```

### 5. フェーズ更新

要求フェーズ状況のフェーズ最終 (公開) を `公開済` に更新する。

### 6. 次工程の案内

- 設計担当が Issue を引き取る際は `/spec-pickup {Issue 番号}` を実行するよう案内
- 設計担当側での取り込みは `状態:設計着手OK` ラベルを検出して行われる

## 注意

- チケットなし開始を許容する (要求文書先行作成)
- ただし公開時点で Issue 化は必須
- ゲート未達で公開拒否した場合、ユーザーに修正対象セクションを具体的に示すこと
