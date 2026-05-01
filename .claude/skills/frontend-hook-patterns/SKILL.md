---
name: frontend-hook-patterns
description: >-
  カスタムフック設計パターン (UI ロジック / 共通フック / API hook 連携 / サイズ制限)。
  "frontend-hook-patterns", "カスタムフック" 等で発動。
---
# Frontend Custom Hooks パターン

## 適用範囲

- `examples/go-react/frontend/`
- `examples/kotlin-nextjs/frontend/`

## 基本設計思想

- カスタムフックは **UI ロジック**を担当する
- **API データ取得 / 更新は生成 hook (Orval) または共通 fetcher を使う** — 手動で書かない
- カスタムフックは生成 hook を**消費する側**として設計

## 配置ルール

| 種類                 | 配置先                       | 例                                                      |
| -------------------- | ---------------------------- | ------------------------------------------------------- |
| 共通 UI フック       | `src/shared/hooks/`          | `useDebounce`, `useAppSnackbar`, `useLocalStorageState` |
| 機能固有 UI ロジック | `src/views/[feature]/hooks/` | `useClientForm`, `useClientFilter`                      |
| API データ取得・更新 | `src/api/generated/` (Orval) | `useGetClients`, `useCreateClient`                      |

## 基本パターン: UI ロジックフック

```typescript
// src/views/clients/hooks/useClientFilter.ts
type ClientFilter = {
  status: string;
  keyword: string;
};

const DEFAULT_FILTER: ClientFilter = { status: "", keyword: "" };

export const useClientFilter = () => {
  const [filter, setFilterState] = useState<ClientFilter>(DEFAULT_FILTER);
  const debouncedKeyword = useDebounce(filter.keyword, 300);

  const setFilter = useCallback((partial: Partial<ClientFilter>) => {
    setFilterState((prev) => ({ ...prev, ...partial }));
  }, []);

  const resetFilter = useCallback(() => setFilterState(DEFAULT_FILTER), []);

  return { filter, setFilter, resetFilter, debouncedKeyword };
};
```

## 生成 hook との組み合わせ

```typescript
// src/views/clients/hooks/useClientList.ts
import { useGetClients } from "@/api/generated/clients";
import { useClientFilter } from "./useClientFilter";

export const useClientList = () => {
  const { filter, setFilter, debouncedKeyword } = useClientFilter();

  const { data, isLoading, error } = useGetClients({
    status: filter.status || undefined,
    keyword: debouncedKeyword || undefined,
    page: 1,
    limit: 20,
  });

  return {
    clients: data?.data ?? [],
    isLoading,
    error,
    filter,
    setFilter,
  };
};
```

## 共通フック例

### useDebounce

```typescript
export const useDebounce = <T>(value: T, delay: number): T => {
  const [debouncedValue, setDebouncedValue] = useState<T>(value);
  useEffect(() => {
    const handler = setTimeout(() => setDebouncedValue(value), delay);
    return () => clearTimeout(handler);
  }, [value, delay]);
  return debouncedValue;
};
```

### useLocalStorageState

```typescript
export const useLocalStorageState = <T>(
  key: string,
  initialValue: T,
): [T, (v: T) => void] => {
  const [state, setState] = useState<T>(() => {
    try {
      const item = localStorage.getItem(key);
      return item !== null ? (JSON.parse(item) as T) : initialValue;
    } catch {
      return initialValue;
    }
  });

  const setValue = useCallback(
    (value: T) => {
      setState(value);
      localStorage.setItem(key, JSON.stringify(value));
    },
    [key],
  );

  return [state, setValue];
};
```

注意: OIDC のリフレッシュトークンは localStorage に保存しない (`/frontend-security` 参照)。

## カスタムフックを作成すべき場合

1. **フォームロジック**: react-hook-form + zod の設定をまとめる
2. **フィルター / ソート状態**: UI 操作に関する状態管理
3. **複数の生成 hook の組み合わせ**: 取得データの加工・統合
4. **ダイアログ開閉**: 状態管理パターンの共通化
5. **100 行以上のコンポーネントから分離すべきロジック**

## 作成すべきでない場合

1. **単純なデータ取得**: 生成 hook をコンポーネントで直接使えば十分
2. **1 行の state**: `useState` をわざわざフック化しない
3. **コンポーネント固有の UI 状態**: ボタンの hover 状態等はコンポーネント内で管理

## サイズ制限

- **1 ファイル最大 200 行**: 超えたら分割
- **単一責任原則**: 1 フック = 1 関心事

## テスト要件

| 配置先                                     | テスト                         |
| ------------------------------------------ | ------------------------------ |
| `src/shared/hooks/`                        | **必須** (カバレッジ 90% 以上) |
| `src/views/[feature]/hooks/` で 100 行以上 | **推奨**                       |
| `src/views/[feature]/hooks/` で 100 行未満 | **任意**                       |
