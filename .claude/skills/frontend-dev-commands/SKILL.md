---
name: frontend-dev-commands
description: >-
  フロントエンドの開発コマンド一覧 (build / lint / format / test / generate / dev)。
  "frontend-dev-commands", "開発コマンド (frontend)" 等で発動。
---
# Frontend 開発コマンド

> **重要**: `dev` 系コマンドは**Claude が直接実行しない**。
> 動作確認が必要な場合はユーザーに実行を依頼する (Hot Module Replacement で長時間プロセスが残るため)。

## 適用範囲

- `examples/go-react/frontend/` (React + Vite)
- `examples/kotlin-nextjs/frontend/` (Next.js)

各サンプルの `package.json` または `Makefile` を真として、以下は標準的な対応関係。

## React + Vite (`examples/go-react/frontend/`)

| コマンド (npm script 例) | 用途 |
|---|---|
| `npm run build` | プロダクション用ビルド (TS コンパイル + Vite ビルド) |
| `npm run lint` | Biome / Oxlint / ESLint のいずれか |
| `npm run lint:fix` | Lint 自動修正 |
| `npm run format` | フォーマットチェック |
| `npm run format:write` | フォーマット修正 |
| `npm run typecheck` | TypeScript 型チェック (`tsc --noEmit`) |
| `npm test` / `npm run test` | Vitest ユニットテスト |
| `npm run test:coverage` | カバレッジレポート |
| `npm run generate` | Orval 実行 (OpenAPI から hooks / 型 / zod 生成) |
| `npm run dev` | **Claude 実行禁止** — 開発サーバー起動 |
| `npm run e2e` | Playwright E2E |

## Next.js (`examples/kotlin-nextjs/frontend/`)

| コマンド | 用途 |
|---|---|
| `npm run build` | プロダクションビルド (`next build`) |
| `npm run lint` | ESLint (`next lint`) |
| `npm run typecheck` | `tsc --noEmit` |
| `npm test` | Vitest / Jest |
| `npm run dev` | **Claude 実行禁止** — `next dev` |
| `npm run start` | プロダクションサーバー起動 |
| `npm run e2e` | Playwright (採用していれば) |

## コミット前チェック順序

```bash
npm run test          # テストが全パス
npm run lint          # Lint エラーなし
npm run format        # フォーマット OK (採用していれば)
npm run typecheck     # 型エラーなし
npm run build         # ビルド成功
```

## 生成コマンドの再実行が必要なタイミング (React + Orval)

- 連携先のバックエンド (`core/api/openapi.yaml` または `examples/go-react/backend/api/openapi.yaml`) が更新された
- `orval.config.ts` を変更した
- API クライアントの mutator / 共通 client 設定を変更した

```bash
npm run generate    # Orval 再生成
npm run build       # 生成後にビルド確認
```

## 禁止事項

- `npm run dev` / `next dev` を Claude が直接実行しない (バックグラウンドプロセスが残るため)
- 生成物 (`src/api/generated/` 等) を手動編集しない
