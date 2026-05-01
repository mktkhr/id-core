---
name: frontend-react-patterns
description: >-
  React コンポーネント設計パターン (Props 駆動 / カスタムフック分離 / ディレクトリ構成 /
  パフォーマンスベストプラクティス)。"frontend-react-patterns", "React 設計" 等で発動。
targets:
  - "*"
---

# Frontend React コンポーネントパターン

## 適用範囲

- `examples/go-react/frontend/` (Vite + React)
- `examples/kotlin-nextjs/frontend/` (Next.js / App Router)

## 設計原則

### 1. コンポーネントサイズ制限

- **1 ファイル最大 800 行** (超えそうなら分割)
- **1 関数最大 50 行** (ロジックはカスタムフックに切り出す)

### 2. Props 駆動の設計

外部依存 (API、グローバル状態) を最小限にし、Props 駆動で動作するコンポーネントを優先する。

```typescript
// ❌ NG: コンポーネント内で API 直接呼び出し (useEffect + setState)
const UserList = () => {
  const [users, setUsers] = useState<User[]>([]);
  useEffect(() => { void fetchUsers().then(setUsers); }, []);
  return <div>{/* ... */}</div>;
};

// ✅ OK: 上位で取得 → Props で子に渡す
type UserListViewProps = { users: User[]; isLoading: boolean };

const UserListView = ({ users, isLoading }: UserListViewProps) => {
  return <div>{/* UI のみ */}</div>;
};

const UserListPage = () => {
  const { data, isLoading } = useGetUsers();
  return <UserListView users={data?.data ?? []} isLoading={isLoading} />;
};
```

### 3. カスタムフックでのロジック分離

UI ロジックはカスタムフックへ。コンポーネントは UI に専念。

```typescript
export const useClientList = () => {
  const { filter, setFilter, debouncedKeyword } = useClientFilter();
  const { data, isLoading } = useGetClients({
    status: filter.status || undefined,
    keyword: debouncedKeyword || undefined,
  });
  return { clients: data?.data ?? [], isLoading, filter, setFilter };
};

const ClientList = () => {
  const { clients, isLoading, filter, setFilter } = useClientList();
  return /* ... */;
};
```

### 4. イミュータビリティ

常に新しいオブジェクトを作成し、ミューテーションしない。

```typescript
// ❌ NG: ミューテーション
const updateUser = (user: User, name: string) => {
  user.name = name;
  return user;
};

// ✅ OK: イミュータブル
const updateUser = (user: User, name: string): User => ({ ...user, name });
```

## ディレクトリ構成

### React + Vite (`examples/go-react/frontend/`)

```
src/views/[feature]/
├── [Feature].tsx       # ページコンポーネント
├── components/         # 機能固有コンポーネント (フラット)
│   ├── ClientCard.tsx
│   └── ClientFilter.tsx
├── hooks/              # 機能固有カスタムフック
│   └── useClientForm.ts
├── types.ts            # UI 固有型
└── schemas.ts          # UI 固有 zod
```

### Next.js (`examples/kotlin-nextjs/frontend/`)

```
src/app/[feature]/
├── page.tsx            # Server Component (デフォルト)
├── layout.tsx
├── components/         # client component / shared
├── hooks/              # client side
├── _server/            # Server Action / Route Handler 等
└── _types.ts
```

App Router の規約に従い、`'use client'` ディレクティブを必要なファイルにのみ付与する。

### フラット構造

- **コンポーネント数 1〜10 個**: フラット構造
- **11 個以上**: ユーザーに相談して分類を判断

ネスト深さ最大 3 階層 (`src/views/[feature]/components/`)。

## ページコンポーネントの責務

ページコンポーネント (`[Feature].tsx`) は以下のみ:

1. カスタムフックからデータ・ロジック取得
2. 子コンポーネントへ Props で渡す
3. レイアウト構成

```typescript
const ClientList = () => {
  const { t } = useTranslation();
  const { clients, isLoading, filter, setFilter } = useClientList();

  return (
    <Box>
      <Typography variant="h5">{t("clients.list.title")}</Typography>
      <ClientFilter filter={filter} onFilterChange={setFilter} />
      {isLoading ? <LoadingOverlay /> : <ClientCardList clients={clients} />}
    </Box>
  );
};
```

## React パフォーマンスベストプラクティス

### A. Autocomplete `onInputChange` にデバウンス必須

```typescript
// ❌ NG: 毎入力で API
const { data } = useListMembers({ q: query || undefined });

// ✅ OK: useDebounce でラップ
const debouncedQuery = useDebounce(query, 300);
const { data } = useListMembers({ q: debouncedQuery || undefined });
```

### B. 導出値を購読する

```typescript
// ❌ NG: 配列全体を購読 (参照が変わるたびに再レンダー)
const items = useStore((s) => s.items);
const isEmpty = items.length === 0;

// ✅ OK: 導出値だけを購読
const isEmpty = useStore((s) => s.items.length === 0);
```

### C. functional setState を使う

```typescript
// ❌ NG: open を購読 → open 変更で再レンダー、依存配列も汚染
const [open, setOpen] = useState(false);
const toggle = useCallback(() => setOpen(!open), [open]);

// ✅ OK: functional setState で安定
const [open, setOpen] = useState(false);
const toggle = useCallback(() => setOpen((prev) => !prev), []);
```

### D. 静的 JSX をコンポーネント外に抽出

```typescript
// ❌ NG: レンダーごとに JSX 再生成
const MyComponent = () => (
  <TextField slotProps={{ input: { startAdornment: <Icon /> } }} />
);

// ✅ OK: 定数化
const startAdornment = <Icon />;
const MyComponent = () => (
  <TextField slotProps={{ input: { startAdornment } }} />
);
```

### E. 依存配列にはプリミティブ値を使う

```typescript
// ❌ NG: filter オブジェクトの参照が毎回変わる
const filter = { status: statusId, category: categoryId };
useEffect(() => {
  fetchData(filter);
}, [filter]);

// ✅ OK: プリミティブに分解
useEffect(() => {
  fetchData({ status: statusId, category: categoryId });
}, [statusId, categoryId]);
```

### F. バレルファイルからの import 禁止 (バンドルサイズ対策)

```typescript
// ❌ NG: バレル経由 (tree-shaking 阻害)
import { Button } from "@/shared/components";

// ✅ OK: 直接 import
import { Button } from "@/shared/components/Button";
```

例外: 外部ライブラリ (MUI / Orval 生成等) のバレルはそのまま使用してよい。

## Snackbar 表示位置規約

| severity            | 表示位置 | anchorOrigin                                  |
| ------------------- | -------- | --------------------------------------------- |
| `error` / `warning` | 上部中央 | `{ vertical: 'top', horizontal: 'center' }`   |
| `success` / `info`  | 右下     | `{ vertical: 'bottom', horizontal: 'right' }` |

詳細は `/frontend-error-patterns` 参照。

## フォーム

詳細は `/frontend-form-patterns` 参照。

- フォームロジックはカスタムフック (`useXxxForm.ts`)
- バリデーションは zod (`schemas.ts`)
- MUI 連携は `Controller`
- バリデーションは送信時のみ (blur 時禁止)
