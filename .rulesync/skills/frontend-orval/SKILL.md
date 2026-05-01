---
name: frontend-orval
description: >-
  Orval 設定・使用パターン (OpenAPI → React Query / 型 / zod 自動生成)。
  "frontend-orval", "Orval", "API 自動生成" 等で発動。
targets:
  - "*"
---

# Frontend Orval ガイド

## 適用範囲

**`examples/go-react/frontend/` (Orval 採用) のみ**

> Next.js (`examples/kotlin-nextjs/frontend/`) は Orval を採用するかは別途決定。
> 採用しない場合、Server Component で `fetch` 直書きまたは別の生成ツール (Hey API 等) を使う。

## 概要

Orval は OpenAPI 仕様から React Query hooks / TypeScript 型 / zod スキーマを自動生成する。
連携先のバックエンド (`core/api/openapi.yaml` または `examples/go-react/backend/api/openapi.yaml`) を参照して生成する。

## 生成コマンド

```bash
npm run generate    # Orval 実行: OpenAPI からコード生成
```

## 生成先

`src/api/generated/` (**手動編集禁止**)

## 設定例

`orval.config.ts` (プロジェクトルート):

```typescript
import { defineConfig } from "orval";

export default defineConfig({
  api: {
    input: {
      // 連携するバックエンドの OpenAPI 仕様を相対パスで指定
      target: "../backend/api/openapi.yaml",
    },
    output: {
      target: "src/api/generated",
      client: "react-query",
      mode: "tags-split",
      httpClient: "fetch",
      override: {
        fetch: { includeHttpResponseReturnType: false },
        mutator: {
          path: "src/api/client.ts",
          name: "customInstance",
        },
        query: {
          useQuery: true,
          useMutation: true,
        },
      },
    },
  },
});
```

## カスタム fetch クライアント (Orval mutator)

```typescript
// src/api/client.ts
export class ApiClientError<ErrorData = unknown> extends Error {
  readonly status?: number;
  readonly response?: { status: number; data: ErrorData; headers: Headers };
}

export type ErrorType<ErrorData> = ApiClientError<ErrorData>;
export type BodyType<BodyData> = BodyData;

export const customInstance = async <T>(
  url: string,
  options?: RequestInit,
): Promise<T> => {
  const response = await fetch(url, options);
  const data = await response.json();

  if (!response.ok) {
    throw new ApiClientError("Request failed", {
      status: response.status,
      response: { status: response.status, data, headers: response.headers },
    });
  }
  return data;
};
```

> **重要**: `fetch` を直接使ってよいのは `src/api/client.ts` のみ。他は生成 hook / 関数を使用する。
> ここで OIDC 認証ヘッダー付与 / 401 リフレッシュ / リダイレクトなどの共通処理を実装する。

## 生成されるコード

### hooks

```typescript
// 自動生成例
export const useGetClients = (
  params?: GetClientsParams,
  options?: UseQueryOptions,
) => {
  return useQuery({
    queryKey: getGetClientsQueryKey(params),
    queryFn: () => getClients(params),
    ...options,
  });
};

export const useCreateClient = (options?: UseMutationOptions) => {
  return useMutation({
    mutationFn: ({ data }) => createClient(data),
    ...options,
  });
};
```

### queryKey factory

```typescript
export const getGetClientsQueryKey = (params?: GetClientsParams) => {
  return ["/api/v1/clients", ...(params ? [params] : [])] as const;
};
```

### TypeScript 型 / zod スキーマ

OpenAPI スキーマから自動生成。UI 固有の型・バリデーションのみ手動定義 (`types.ts`, `schemas.ts`)。

## 使用パターン

### useQuery

```typescript
import { useGetClients } from "@/api/generated/clients";

const ClientList = () => {
  const { data, isLoading, error } = useGetClients({ page: 1, limit: 20 });
  if (isLoading) return <LoadingOverlay />;
  if (error) return <ErrorDisplay error={error} />;
  return <ClientListView clients={data?.data ?? []} />;
};
```

### useMutation + キャッシュ無効化

```typescript
import {
  getGetClientsQueryKey,
  useCreateClient,
} from "@/api/generated/clients";
import { useQueryClient } from "@tanstack/react-query";
import { useAppSnackbar } from "@/shared/hooks/useAppSnackbar";

const useClientForm = () => {
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

### キャッシュ無効化は queryKey factory を使う

```typescript
queryClient.invalidateQueries({ queryKey: getGetClientsQueryKey() });
// ❌ NG: 文字列で書かない
queryClient.invalidateQueries({ queryKey: ["/api/v1/clients"] });
```

## 禁止事項

1. **`src/api/generated/` の手動編集禁止** → `npm run generate` で再生成
2. **手動で API 関数を書かない** → 生成物を使う
3. **手動で queryKey を定義しない** → key factory を使う
4. **`fetch` を直接 import しない** (`src/api/client.ts` のみ例外)
5. **生成 hook の options で `queryKey` を上書きしない** → キャッシュ整合性が壊れる

## 再生成が必要なタイミング

- 連携先バックエンドの OpenAPI 仕様が更新された
- `orval.config.ts` を変更した
- `src/api/client.ts` の mutator 署名を変更した

```bash
npm run generate
npm run build
```
