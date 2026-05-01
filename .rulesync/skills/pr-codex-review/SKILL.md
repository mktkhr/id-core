---
name: pr-codex-review
description: >-
  PR 番号を引数に取り、diff + description を Codex に投げてレビューさせる。
  PR 作成直後に必ず実行する (詳細は .rulesync/rules/pr-review-policy.md)。
  "PR レビュー", "pr-codex-review", "Codex で PR レビュー", "PR をレビューして" 等で発動。
targets:
  - "*"
---

# PR Codex Review

PR の **diff + description** を Codex に投げてレビューさせる。
**全ての PR は本スキル (または同等の `/ask-codex` 直接呼び出し) でレビューゲートを通過してからマージする**
(`.rulesync/rules/pr-review-policy.md` §1 参照)。

## 使い方

```
/pr-codex-review {PR 番号}
```

引数なしで呼び出された場合は、現在のブランチに紐付く PR 番号を `gh pr view --json number --jq .number` で取得する。

## 手順

### 1. 実行前チェック (必須)

- 現在が Plan mode の場合、ユーザーに次を確認:
  - `Plan mode を終了して Codex 委譲を実行しますか? (Y/N)`
- 承認後 `ExitPlanMode` を実行
- `which codex` で Codex CLI の存在を確認 (なければエラーで停止)
- `gh auth status` で gh CLI の認証を確認

### 2. PR 情報の取得

```bash
PR_NUM="${ARGUMENTS:-$(gh pr view --json number --jq .number)}"
PR_DIFF=$(gh pr diff "$PR_NUM")
PR_BODY=$(gh pr view "$PR_NUM" --json title,body --jq '"## " + .title + "\n\n" + .body')
PR_FILES=$(gh pr view "$PR_NUM" --json files --jq '.files | map(.path) | join("\n")')
```

### 3. Codex 委譲

```bash
mkdir -p .ai-out
OUTPUT_FILE=".ai-out/codex-pr-${PR_NUM}-review-$(date +%Y%m%d%H%M%S).md"

PROMPT="あなたは Pull Request レビュアです。以下の PR をレビューしてください。

== PR description ==

${PR_BODY}

== 変更ファイル ==

${PR_FILES}

== PR diff ==

\`\`\`diff
${PR_DIFF}
\`\`\`

== レビュー観点 ==

リポジトリのルールは .rulesync/rules/pr-review-policy.md に集約されています。特に以下の観点を厳格にチェックしてください。

1. **アーカイブ参照や固有事情の混入** (公開リポジトリ前提、テンプレートとして他者再利用される)
   - 'アーカイブ', 'ナンダカンダ', 'asset-management' 等の特定リポジトリ名 / 社内事情の文言があれば指摘
2. **誤メンションを引き起こす裸の @ 表記**
   - 'Action @master', '@v2', '@latest' 等が PR description でバッククォート外に書かれていないか
3. **ランタイム / 依存 / Action のバージョン**
   - go.mod / package.json / Dockerfile / Action の uses で古いバージョンが使われていないか
   - 'CI を通すためにバージョンを下げる' 系の独断変更は CRITICAL 扱い
4. **差分が PR スコープに収まっているか** (関係ないファイルが混ざっていないか)
5. **PR description と差分の整合**
6. **重複コード / 命名違反 / 規約違反**
7. **テスト網羅 / カバレッジ低下** (該当する場合)
8. **セキュリティ観点** (シークレット混入、入力検証欠落、認可漏れ等)

CRITICAL / HIGH / MEDIUM / LOW で重要度別に問題を列挙してください。各問題について該当箇所と修正案を示すこと。

最後に必ず ## Summary セクションを作成し、重要度別件数 + マージ可否判定 (gate: CRITICAL=0/HIGH=0/MEDIUM<3) を簡潔にまとめること。"

codex exec --full-auto "\$PROMPT" > "\$OUTPUT_FILE" 2>&1
```

### 4. Summary 抽出

Codex 出力には PROMPT 内の `## Summary` 文字列も含まれるため、**最後の `## Summary` セクション**を抽出する:

```bash
awk '/^## Summary/{c++} END {print c}' "$OUTPUT_FILE"   # 出現回数
awk '/^## Summary/{c++} c>=N {print}' "$OUTPUT_FILE"    # N 番目以降を抽出 (N は出現回数)
```

または、ファイル末尾から逆引きして最後の `## Summary` を抽出する:

```bash
tac "$OUTPUT_FILE" | awk '/^## Summary/{found=1} found' | tac
```

### 5. ゲート判定

Summary の重要度別件数を読み取り、以下のゲートを判定:

| 件数                                   | 判定                             |
| -------------------------------------- | -------------------------------- |
| CRITICAL ≥ 1                           | ❌ 不通過 (ブロッカー、必ず修正) |
| HIGH ≥ 1                               | ❌ 不通過 (修正必須)             |
| MEDIUM ≥ 3                             | ❌ 不通過 (修正推奨)             |
| CRITICAL=0 かつ HIGH=0 かつ MEDIUM ≤ 2 | ✅ 通過 (マージ可)               |

### 6. ユーザー報告

- ゲート結果と件数を表形式で要約
- CRITICAL / HIGH があれば各項目を引用 (修正対象)
- MEDIUM ≤ 2 の場合は対応するか後続改善で残すかをユーザーに提案
- 全文の保存先 (`.ai-out/codex-pr-{番号}-review-{日時}.md`) を併記

### 7. 修正サイクル (ゲート未達時)

1. 指摘を反映 (コミット)
2. push
3. 本スキルを再度実行 (`/pr-codex-review {同じ PR 番号}`)
4. ゲート通過まで繰り返す

## 注意

- Codex の出力全文は読み込まない (コンテキスト保護)
- レビュー結果ファイル (`.ai-out/`) は `.gitignore` で除外済み
- バックグラウンド実行が必要な大きな PR では `frontend-codex-review` のように `run_in_background: true` 化を検討

## 関連

- ルール: `.rulesync/rules/pr-review-policy.md`
- 関連スキル: `/ask-codex` (汎用 Codex 委譲)、`/frontend-codex-review` (フロントエンド差分のバックグラウンドレビュー)
