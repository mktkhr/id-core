---
name: frontend-architecture
description: >-
  フロントエンドのディレクトリ構造・コンポーネント配置・フック配置・レイヤールール。 "frontend-architecture", "フロント構造"
  等で発動。
---
# Frontend Architecture Guide

## 適用範囲

- `examples/go-react/frontend/` (Vite + React)
- `examples/kotlin-nextjs/frontend/` (Next.js / App Router)

両方で**通用する原則**を共通とし、フレームワーク固有の構成は分けて記載する。

## ディレクトリ構造 (React + Vite)

```
src/
├── api/
│   ├── client.ts              # カスタム fetch クライアント (Orval mutator)
│   └── generated/             # Orval 生成 (手動編集禁止)
├── layouts/                   # レイアウトコンポーネント
│   ├── AppLayout.tsx
│   └── AuthLayout.tsx
├── router/                    # ルート定義
│   └── index.tsx              # createBrowserRouter
├── shared/                    # 2 つ以上の feature で使用する共通コード
│   ├── components/
│   ├── hooks/
│   ├── utils/                 # 純粋関数 (テスト 100%)
│   └── types/
├── views/
│   └── [feature]/
│       ├── [Feature].tsx      # ページコンポーネント
│       ├── components/        # 機能固有コンポーネント (フラット)
│       ├── hooks/             # 機能固有カスタムフック
│       ├── types.ts
│       └── schemas.ts
├── locales/
│   ├── ja.json
│   └── en.json
├── theme/                     # MUI テーマ等
├── App.tsx                    # Provider 構成 + RouterProvider
└── main.tsx
```

## ディレクトリ構造 (Next.js / App Router)

```
src/
├── app/                       # App Router (route 単位)
│   ├── (authenticated)/
│   │   ├── clients/
│   │   │   ├── page.tsx       # Server Component
│   │   │   ├── _components/
│   │   │   ├── _hooks/
│   │   │   └── _server/       # Server Action / Route Handler
│   │   └── layout.tsx
│   ├── auth/
│   ├── api/                   # Route Handler 群
│   ├── globals.css
│   └── layout.tsx             # Root Layout
├── shared/
│   ├── components/
│   ├── hooks/
│   └── utils/
├── lib/                       # 設定・SDK 初期化
├── messages/                  # next-intl 等
└── types/
```

## コンポーネント配置ルール (共通)

### 判断フロー

```
新コンポーネント
  ↓
1 feature 内のみ → src/views/[feature]/components/  (Vite)
                    src/app/[route]/_components/   (Next.js)
2+ feature で使用 → src/shared/components/

ページレベル/ルートのレイアウト → src/layouts/ (Vite)
                                src/app/.../layout.tsx (Next.js)
```

### フラット構造

`components/` 内は**フラット構造**。サブディレクトリは原則作らない。

```
# ✅ OK
components/
├── ClientCard.tsx
├── ClientFilter.tsx
└── ClientDetailDialog.tsx

# ❌ NG: 不要なネスト
components/
├── Card/ClientCard.tsx
└── Filter/ClientFilter.tsx
```

### ディレクトリ化の判断

| コンポーネント数 | 構造                        |
| ---------------- | --------------------------- |
| 1〜10            | フラット                    |
| 11+              | **ユーザーに相談** して分類 |

自己判断で構造変更しない。

## フック配置ルール

### 判断フロー

```
フック作成
  ↓
API データ取得・更新? → 手動作成しない。生成 hook (Orval) または fetch ラッパーを使う
  ↓
2+ feature で使用? → YES: src/shared/hooks/
                     NO:  src/views/[feature]/hooks/
```

### 配置先

| 種類                 | 配置先                                          | 例                                 |
| -------------------- | ----------------------------------------------- | ---------------------------------- |
| API データ取得・更新 | `src/api/generated/` (Orval) または共通 fetcher | `useGetClients`, `useCreateClient` |
| 共通 UI フック       | `src/shared/hooks/`                             | `useDebounce`, `useAppSnackbar`    |
| 機能固有 UI ロジック | `src/views/[feature]/hooks/`                    | `useClientForm`, `useClientFilter` |

### フックの責務

feature 内のフックは **UI ロジックのみ**:

- フォーム状態管理 (react-hook-form 連携)
- ダイアログ開閉
- フィルター / ソート状態
- 生成 hook の結果を組み合わせた UI ステート

## レイアウトとルーター (React + Vite)

```tsx
// src/router/index.tsx
const router = createBrowserRouter([
  {
    element: <AppLayout />,
    children: [
      { path: "/clients", element: <ClientList /> },
      { path: "/clients/:id", element: <ClientDetail /> },
      { path: "/profile", element: <Profile /> },
    ],
  },
  {
    element: <AuthLayout />,
    children: [{ path: "/login", element: <Login /> }],
  },
]);
```

### Provider 構成 (例)

```tsx
const App = () => (
  <QueryClientProvider client={queryClient}>
    <ThemeProvider theme={theme}>
      <I18nextProvider i18n={i18n}>
        <SnackbarProvider>
          <RouterProvider router={router} />
        </SnackbarProvider>
      </I18nextProvider>
    </ThemeProvider>
  </QueryClientProvider>
);
```

Provider 順序の根拠:

1. `QueryClientProvider` — 最外側。全コンポーネントから React Query 利用可
2. `ThemeProvider` — MUI テーマ
3. `I18nextProvider` — 翻訳
4. `SnackbarProvider` — グローバル通知
5. `RouterProvider` — 最内側

## App Router (Next.js)

- **Server Component** がデフォルト。`'use client'` は必要なファイルにのみ
- データ取得は Server Component で `fetch()` (Next.js 拡張のキャッシュ・revalidate を活用)
- 認証付きルートは `(authenticated)` の route group で囲む
- ミューテーションは Server Action または Route Handler で

## 新機能追加フロー (React + Vite)

1. 連携先バックエンドの OpenAPI 仕様が更新されたら `npm run generate` で Orval 再生成
2. ディレクトリ作成:
   ```bash
   mkdir -p src/views/<feature>/{components,hooks}
   touch src/views/<feature>/{types.ts,schemas.ts}
   ```
3. 実装順序:
   1. `types.ts` — UI 固有の型 (API 型は生成済み)
   2. `schemas.ts` — zod スキーマ (UI 固有)
   3. `hooks/` — カスタムフック (UI ロジック)
   4. `components/` — コンポーネント (フラット)
   5. `[Feature].tsx` — ページコンポーネント
4. i18n: `ja.json` + `en.json` 同時追加
5. ルーター: `src/router/index.tsx` にルート追加
6. テスト: 純粋関数必須、フック推奨
7. 品質: `npm run test` → `npm run lint` → `npm run typecheck` → `npm run build`

## 新機能追加フロー (Next.js)

1. ルートディレクトリ作成: `src/app/<route>/page.tsx`
2. `_components/`, `_hooks/`, `_server/` を必要に応じて作成
3. 実装順序:
   1. Server Component で初期データ取得
   2. Client Component でインタラクション
   3. Server Action / Route Handler でミューテーション
4. i18n 同時更新
5. テスト
6. 品質: `npm run lint` → `npm run typecheck` → `npm run build`

## 共有コードの判断基準

`shared/` に入れる基準: **2 つ以上の feature で使用される**もの。

| 最初は 1 feature のみ                                              | 2 feature で使う時 |
| ------------------------------------------------------------------ | ------------------ |
| `views/[feature]/` (Vite) または `app/[route]/_*` (Next.js) に配置 | `shared/` に移動   |

**過度な先回り抽象化禁止**。最初から `shared/` に入れるのは明らかに汎用的なもの (Dialog / Loading / Snackbar 等) のみ。

## インポート順序 (Lint で自動整列するが論理的順序)

1. React / React DOM / Next.js
2. 外部ライブラリ (MUI / react-router / react-query / next-intl 等)
3. `@/shared/` (共通コード)
4. `@/views/` または `@/app/` (同一 feature 内)
5. `@/api/` (API 層)
6. 型のみ import (`type` キーワード付き)

## 環境変数

- React + Vite: `VITE_` プレフィックス、ビルド時にバンドル埋め込み
- Next.js: クライアント公開は `NEXT_PUBLIC_` プレフィックス、サーバー専用は接頭辞なし

**真の秘密値はバックエンドに置く**。フロントの環境変数はブラウザから読める前提で扱う。

## OIDC RP の構造的留意点

サンプルアプリは id-core (OP) を上流とする RP として動作する想定。

- 認可コードフロー + PKCE
- Server Side で `client_secret` を保管 (Next.js なら Server Component / Route Handler、Vite なら BFF を別途用意)
- ブラウザにはアクセストークンのみを短命で保持
- リフレッシュトークン rotation はサーバー側で実施 (HttpOnly Cookie 経由が理想)
