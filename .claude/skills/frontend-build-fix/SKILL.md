---
name: frontend-build-fix
description: >-
  フロントエンドのビルド/Lint/型エラーを段階的に修正する。"frontend-build-fix", "ビルドエラー", "lint エラー
  (frontend)" 等で発動。
---
# Frontend Build & Fix

フロントエンド (`examples/*/frontend/`) のビルド・Lint・型エラーを段階的に修正する。

## 適用範囲

- `examples/go-react/frontend/` (React + Vite + MUI + Orval 想定)
- `examples/kotlin-nextjs/frontend/` (Next.js 想定)

## 手順

1. ビルド/Lint 実行 (例)
   - React (Vite): `npm run build` / `npm run lint` / `npm run typecheck`
   - Next.js: `npm run build` / `npm run lint`
2. エラー出力を解析: ファイルごとにグループ化、重要度順
3. 各エラーに対して:
   - エラーコンテキストを表示 (前後 5 行)
   - 問題を説明 → 修正案 → 適用 → 再実行
4. 以下の場合は停止:
   - 修正が新しいエラーを導入
   - 同じエラーが 3 回連続で残る
   - ユーザーが一時停止を要求
5. サマリー: 修正数 / 残存数 / 新規導入数

## TypeScript エラー

**型推論失敗:**

- ジェネリクスを明示: `useState<string[]>([])`
- 型ガード関数を実装

**`any` 検出:**

- `unknown` + 型ガードに変更
- 適切な型を追加

**null / undefined チェック:**

- オプショナルチェーン `?.`
- `??` でデフォルト値
- **非 null アサーション `!` は使用禁止**

## React エラー

**コンポーネント解決失敗:**

- import パスを確認 (プロジェクト規約に従う。`@/` 絶対パス推奨)

**Props 型エラー:**

- Props 型定義を確認、required / optional を確認

**Hooks 違反:**

- Hooks はコンポーネントまたはカスタムフックのトップレベルでのみ使用

## Lint エラー (Biome / Oxlint / ESLint いずれも)

**未使用変数:**

- 不要なら削除 (`_` プレフィックスで意図的を表明する場合あり)

**import 順序:**

- 自動修正コマンドを実行 (Biome: `biome format --write`、ESLint: `eslint --fix`)

**`console.log`:**

- すべて削除。意図的な出力は `console.error` / `console.warn` のみ

**相対パス:**

- プロジェクトの規約 (`@/...` 絶対パス) に変更

## Orval (該当時)

- `npm run generate` で再生成
- `src/api/generated/` は**手動編集禁止**

## Next.js 固有

- `next/font` のロードエラー: フォントパス・variable 設定を確認
- App Router の `'use client'` ディレクティブ漏れ
- Server Component と Client Component の境界違反

## 原則

- **一度に 1 つのエラーを修正**して再実行する
- ビルドが通らないコードをコミットしない
