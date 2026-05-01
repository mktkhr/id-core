---
name: ask-codex
description: >-
  Codex CLI に質問や分析タスクを委譲して実行する。重い分析やリサーチ時に使用。 "ask-codex", "Codex に聞いて", "Codex
  に委譲", "重い分析" 等で発動。
---
# Codex 委譲スキル

Codex CLI に質問・分析・全文レビューを委譲して実行する。
詳細ルールは `.rulesync/rules/codex-delegation.md` に従う。

## 手順

### 1. 実行前チェック (必須)

- 現在が Plan mode の場合、以下をユーザーに確認する:
  - `Plan mode を終了して Codex 委譲を実行しますか? (Y/N)`
- ユーザーの明示承認が得られるまで、委譲を実行しない
- 承認された場合のみ `ExitPlanMode` を実行してから次に進む

### 2. セッション継続判定と実行

`.ai-out/.codex-session-id` が存在するか確認し、実行コマンドを切り替える:

```bash
mkdir -p .ai-out
SESSION_FILE=".ai-out/.codex-session-id"
OUTPUT_FILE=".ai-out/codex-$(date +%Y%m%d%H%M%S).md"
PROMPT="$ARGUMENTS

最後に必ず '## Summary' セクションを作成し、主要な発見事項・重要度別分類 (CRITICAL/HIGH/MEDIUM/LOW)・推奨アクションを日本語で簡潔にまとめること。"

if [ -f "$SESSION_FILE" ]; then
  SESSION_ID=$(cat "$SESSION_FILE")
  codex exec resume "$SESSION_ID" --full-auto "$PROMPT" > "$OUTPUT_FILE" 2>&1
else
  codex exec --full-auto "$PROMPT" > "$OUTPUT_FILE" 2>&1
  tail -1 ~/.codex/session_index.jsonl 2>/dev/null \
    | python3 -c "import sys,json; print(json.loads(sys.stdin.read().strip())['id'])" \
    > "$SESSION_FILE" 2>/dev/null || true
fi
```

### 3. Summary セクションのみ読み取り

```bash
sed -n '/^## Summary/,$p' "$OUTPUT_FILE"
```

- Codex の出力はコンテキスト保護のため**全文を読まない**
- Summary だけでは判断できない場合のみ、必要な箇所をピンポイントで読む

### 4. ユーザー報告

- Summary の内容を要約してユーザーに報告
- 全文の保存先 (`.ai-out/codex-{日時}.md`) を併記する

## 委譲タスクの目安

詳細は `.rulesync/rules/codex-delegation.md` を参照。

- 設計書 / OpenAPI / マイグレーションの全文レビュー
- 認可マトリクス ↔ 実装の整合性チェック
- 横断的な調査・grep + 解釈
- セカンドオピニオン

## 禁止

- Codex に書き込み権限を与えるタスクを委譲しない
- リポジトリ外の絶対パスを Codex プロンプトに含めない
- 結果ファイル全文を Claude Code のコンテキストに読み込まない
