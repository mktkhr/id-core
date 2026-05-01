---
name: frontend-code-review
description: >-
  React + TypeScript フロントエンドのコードレビューを実施する。"frontend-code-review",
  "フロントレビュー", "React レビュー" 等で発動。
targets:
  - "*"
---

# Frontend Code Review

## 使い方

```
/frontend-code-review              # ステージング + 未ステージング差分
/frontend-code-review main         # main との差分
/frontend-code-review HEAD~3       # 直近 3 コミット
```

## 手順

1. 差分の取得:
   - 引数なし → `git status` + `git diff` (ステージング + 未ステージング)
   - 引数あり → `git diff ${branch}` でそのブランチ・リビジョンとの差分
2. `src/api/generated/` (Orval 生成) はレビュー対象外
3. 以下の観点で重要度別レポート

## レビュー観点

### TypeScript 安全性 (CRITICAL)

詳細は `/frontend-ts-safety` 参照。

- `any` の使用 → `unknown` + 型ガード
- `as` 型アサーション → 型ガード関数
- `!` 非 null アサーション → `?.` / `??` / 事前ガード
- `interface` の使用 → `type`
- `function` 宣言の使用 → アロー関数
- `biome-ignore` / `@ts-ignore` の使用
- Index Signature `[key: string]` → 定義型 or `Record`
- catch 句で `unknown` を使っていない

### セキュリティ (CRITICAL)

詳細は `/frontend-security` 参照。

- ハードコードされたシークレット (API キー、パスワード、トークン)
- `dangerouslySetInnerHTML` の使用
- `eval()` / `new Function()` の使用
- ユーザー入力のバリデーション欠如
- localStorage に OIDC トークン全文保存
- `target="_blank"` で `rel="noopener noreferrer"` なし

### アーキテクチャ (HIGH)

- 直接 `fetch` / HTTP クライアントの使用 → 生成 hook または共通クライアント必須
- 相対 import の使用 → `@/` プレフィックス
- `src/api/generated/` の手動編集
- ミューテーション (オブジェクト・配列の直接変更) → スプレッド構文必須

### コンポーネント設計 (HIGH)

- コンポーネント / ファイルが 800 行超
- カスタムフックが 200 行超
- 関数が 50 行超
- 4 レベル以上のネスト
- ビジネスロジックがコンポーネント内に混在 → カスタムフック化
- Props drill が深すぎる

### スタイリング (HIGH) — MUI 採用時

- `makeStyles` / `withStyles` の使用 → `sx` プロパティ
- ハードコードされたカラーコード → テーマトークン (`primary.main` 等)
- `vh` / `dvh` / `svh` / `lvh` の使用 → `src/layouts/` のみ例外
- Snackbar 表示位置:
  - `error` / `warning` → 上部中央
  - `success` / `info` → 右下

### コード品質 (HIGH)

- エラーハンドリングの欠如 (async に try/catch なし)
- `console.log` の存在 → `console.error` / `console.warn` のみ
- 全角括弧 `（）` の混入

### i18n (MEDIUM)

- ハードコードされた日本語/英語文字列 → `t()` 経由
- `ja.json` のみ更新 (`en.json` 未更新) → 同時更新必須
- 動的キー構築

### テスト (MEDIUM)

- テストの `skip` による回避禁止
- 純粋関数 (`src/shared/utils/`) のテスト欠如
- 共通フック (`src/shared/hooks/`) のテスト欠如

### パフォーマンス (MEDIUM)

詳細は `/frontend-react-patterns` 参照。

- 不要な `useEffect` (派生値は `useMemo`)
- 大きなリストで `key` 不在 / 不適切な `key` (index 使用)
- 静的 JSX をコンポーネント外に抽出していない
- バレルファイル経由の import (tree-shaking 阻害)

## レポート生成

各問題に:

- 重要度 (CRITICAL / HIGH / MEDIUM / LOW)
- ファイル位置と行番号
- 問題の説明
- 修正案

CRITICAL または HIGH が見つかった場合はコミットをブロック。

## 禁止

- git config / リポジトリ設定の変更を提案しない
- コミット / プッシュを実行しない
- セキュリティ脆弱性を見逃して承認しない
