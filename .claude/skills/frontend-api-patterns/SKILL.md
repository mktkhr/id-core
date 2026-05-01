---
name: frontend-api-patterns
description: >-
  フロントエンドの API 通信パターン (Orval / 共通クライアント / キャッシュ無効化)。 "frontend-api-patterns",
  "API 通信", "fetch 直叩き禁止" 等で発動。
---
# Frontend API パターン

API 通信の実装パターン。

## 適用範囲

- `examples/go-react/frontend/` (Orval + React Query 想定)
- `examples/kotlin-nextjs/frontend/` (App Router の Server Component / Route Handler または fetch + SWR 想定)

## 基本方針 (共通)

1. **fetch を直接呼ばない** (共通クライアント / Orval 経由)
2. **API 関数を手動で書かない** (生成物または共通ファクトリを使う)
3. **queryKey を手動で定義しない** (Orval の key factory または共通命名規約を使う)
4. **Snackbar フィードバック必須** (mutation 成功・失敗ともに `useAppSnackbar`、サイレント失敗禁止)

## React + Vite + Orval (`examples/go-react/frontend/`)

詳細は `/frontend-orval` 参照。

### ディレクトリ構成

```
src/api/
├── client.ts              # カスタム fetch クライアント (Orval mutator)
└── generated/             # Orval 生成物 (手動編集禁止)
    ├── auth.ts
    ├── clients.ts
    └── ...
```

### データ取得 (生成 hook をそのまま使用)

```typescript
import { useGetClients } from "@/api/generated/clients";

const ClientList = () => {
  const { data, isLoading, error } = useGetClients({ page: 1, limit: 20 });

  if (isLoading) return <LoadingOverlay />;
  if (error) return <ErrorDisplay error={error} />;

  return <ClientListView clients={data?.data ?? []} />;
};
```

### カスタムフックでの組み合わせ

複数 query を組み合わせる場合や UI ロジックが絡む場合は hook 化:

```typescript
import { useGetClient } from "@/api/generated/clients";
import { useGetAuthCodesByClient } from "@/api/generated/authCodes";

export const useClientDetail = (clientId: string) => {
  const { data: client, isLoading: c1 } = useGetClient(clientId);
  const { data: codes, isLoading: c2 } = useGetAuthCodesByClient(clientId);

  return {
    client,
    authCodes: codes?.data ?? [],
    isLoading: c1 || c2,
  };
};
```

### Mutation + キャッシュ無効化 + Snackbar

```typescript
import { getGetClientsQueryKey, useCreateClient } from "@/api/generated/clients";
import { useAppSnackbar } from "@/shared/hooks/useAppSnackbar";
import { useQueryClient } from "@tanstack/react-query";

const useClientCreate = () => {
  const queryClient = useQueryClient();
  const { showSuccess, showError } = useAppSnackbar();

  return useCreateClient({
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: getGetClientsQueryKey() });
      showSuccess(t("clients.create.success"));
    },
    onError: (error) => {
      const apiError = extractApiError(error);
      showError(apiError.message);
    },
  });
};
```

### Snackbar の責務配置

- ダイアログ内 mutation の `onSuccess` を親に伝える設計では、**親 (一覧画面など) でまとめて Snackbar 表示** (重複防止)
- 詳細画面から一覧へ `navigate` する場合、`navigate` 前に `showSuccess` (Provider はルート直下なのでアンマウント後もトーストは残る)
- Combobox での新規作成のように hook 内部で mutation するケースは、**hook 側で `showSuccess`** (呼び出し側に責務を分散しない)

## Next.js (`examples/kotlin-nextjs/frontend/`)

### App Router での標準パターン

- **Server Component**: 直接 `fetch()` (Next.js が拡張した自動キャッシュ・revalidate 機能を使う)
- **Route Handler / Server Action**: ミューテーション系。サーバー側で id-core API を呼ぶ
- **Client Component**: SWR / React Query で `/api/...` (自前 Route Handler) を叩く

### Server Component での fetch

```typescript
// app/(authenticated)/clients/page.tsx
async function getClients() {
  const res = await fetch(`${process.env.API_BASE_URL}/clients`, {
    headers: { Authorization: `Bearer ${await getAccessToken()}` },
    next: { revalidate: 60, tags: ["clients"] },
  });
  if (!res.ok) throw new Error("クライアント一覧の取得に失敗しました");
  return res.json();
}
```

### Server Action での mutation

```typescript
"use server";

import { revalidateTag } from "next/cache";

export async function createClient(input: CreateClientInput) {
  const res = await fetch(`${process.env.API_BASE_URL}/clients`, {
    method: "POST",
    body: JSON.stringify(input),
    headers: { /* ... */ },
  });
  if (!res.ok) throw new Error("作成に失敗しました");
  revalidateTag("clients");
}
```

## エラーハンドリング (共通)

```typescript
import { isApiClientError } from "@/api/client";
import { extractApiError } from "@/api/common";

const handleApiError = (error: unknown) => {
  if (isApiClientError(error) && error.status === 409) {
    showError(t("common.error.conflict"));
    return;
  }
  const apiError = extractApiError(error);
  showError(apiError.message);
};
```

## グローバル処理 (共通クライアント)

`src/api/client.ts` (React) または共通 fetch ラッパー (Next.js) で:

- Authorization ヘッダー付与 (id-core 発行のアクセストークン)
- 401 時のリフレッシュトークン rotation + 待機キュー
- リフレッシュ失敗時のトークンクリア + `/login` リダイレクト

## 禁止パターン

```typescript
// ❌ NG: fetch を直接インポートしてバックエンドを叩く (Server Component を除く)
const res = await fetch("/api/v1/clients");

// ❌ NG: API 関数を手動で書く (共通ファクトリを使う)
export const getClients = async () => customInstance("/api/v1/clients", { method: "GET" });

// ❌ NG: queryKey を手動で定義
const { data } = useQuery({ queryKey: ["clients"], queryFn: fetchClients });

// ❌ NG: useEffect + setState でデータ取得 (React Query / SWR を使う)
const [data, setData] = useState([]);
useEffect(() => { void fetchClients().then(setData); }, []);

// ✅ OK: 生成 hook
const { data } = useGetClients();
```
