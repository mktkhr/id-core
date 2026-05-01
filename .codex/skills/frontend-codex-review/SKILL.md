---
name: frontend-codex-review
description: >-
  フロントエンド差分の Codex レビューをバックグラウンド実行する。"frontend-codex-review", "フロントの Codex レビュー"
  等で発動。
---
# Frontend Codex Review

フロントエンド (`examples/*/frontend/`) の差分を Codex に委譲し、バックグラウンドで実行する。

## 使い方

```
/frontend-codex-review              # main との差分をレビュー
/frontend-codex-review HEAD~3       # 直近 3 コミットの差分をレビュー
```

## 手順

1. **実行前チェック**
   - Plan mode ならユーザー確認 → `ExitPlanMode`

2. **比較対象決定**
   - 引数あり: そのブランチ / リビジョン
   - 引数なし: `main`

3. **Bash の `run_in_background: true` で実行**

   ```bash
   mkdir -p .ai-out
   BASE_REF="${引数:-main}"
   OUTPUT_FILE=".ai-out/codex-frontend-review-$(date +%Y%m%d%H%M%S).md"

   codex exec --full-auto \
     "git diff ${BASE_REF} の変更内容をフロントエンド観点でレビューせよ。
     対象: examples/*/frontend/ 配下。生成物 (src/api/generated/ 等) はスキップ。
     観点:
     - any / as / 非 null assertion (!) / interface (type 推奨) の混入
     - 相対 import / 直接 fetch (Orval / 共通クライアント経由でない) の混入
     - dangerouslySetInnerHTML / console.log の混入
     - UI テキストのハードコード (i18n キー化されているか)
     - data-testid 命名規則違反
     - ハードコードカラー / vh|dvh の使用 (layout 系のみ例外)
     - ミューテーション禁止 (props / state を直接書き換えていないか)
     - Snackbar 配置規則 (error/warning=上部中央, success/info=右下)
     - useEffect の不必要使用 (派生値は useMemo で計算)
     - i18n 同時更新 (ja/en 両方)
     各問題に CRITICAL/HIGH/MEDIUM/LOW・ファイル・行番号・修正案。
     最後に必ず ## Summary を日本語で作成。" \
     < /dev/null > "$OUTPUT_FILE" 2>&1

   echo "保存先: $OUTPUT_FILE"
   ```

   - `timeout: 600000` (最大 10 分) を設定

4. **ユーザーへの初期報告**

   「Codex レビューをバックグラウンドで実行中です。完了したら報告します。」

5. **完了後、Summary のみ読み取り**

   ```bash
   sed -n '/^## Summary/,$p' "$OUTPUT_FILE"
   ```

   - 全文は読まない (コンテキスト保護)
   - Summary だけで判断できない場合のみ、必要箇所をピンポイントで読む

6. **ユーザーへ要約報告 + 保存先パスを併記**

## 注意

- `codex exec` は Bash で直接実行 (サブエージェント委譲しない)
- 必ず `run_in_background: true`
- 結果は `.ai-out/` 配下に保存し、コミット対象にしない
