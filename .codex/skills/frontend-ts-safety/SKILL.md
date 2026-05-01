---
name: frontend-ts-safety
description: >-
  TypeScript の型安全性ルール (any / as / ! 禁止 / 型ガード / interface vs type)。
  "frontend-ts-safety", "型安全", "TypeScript" 等で発動。
---
# Frontend TypeScript 安全性

型安全性を最優先し、実行時エラーを防ぐ。

## 適用範囲

- `examples/go-react/frontend/`
- `examples/kotlin-nextjs/frontend/`

## 禁止事項

### 1. Lint / TS Ignore の禁止

`@ts-ignore` / `@ts-expect-error` / `biome-ignore` / `eslint-disable` 等で**強引に通過する** ことを禁止。

```typescript
// ❌ NG
// @ts-ignore
const result = unsafeOperation();

// ✅ OK: 型を正確に定義
type UnsafeResult = { success: boolean; data?: unknown };
const result: UnsafeResult = unsafeOperation();
```

例外: ライブラリ起因の不可避な型不整合のみ、コメントで理由を明記して許可。

### 2. `any` の完全禁止 (暗黙的 any も)

具体的な型を使う。型不明な場合は `unknown` + 型ガード。

```typescript
// ❌ NG
const processData = (data) => data.value;

// ✅ OK
type Data = { value: string };
const processData = (data: Data) => data.value;

// ✅ OK: unknown + 型ガード
const processUnknown = (data: unknown) => {
  if (typeof data === "object" && data !== null && "value" in data) {
    return (data as { value: unknown }).value;
  }
  throw new Error("Invalid data format");
};
```

### 3. 型アサーション (`as`) の完全禁止

型ガード関数を使う。

```typescript
// ❌ NG
const user = response.data as User;

// ✅ OK
const isUser = (data: unknown): data is User => {
  return (
    typeof data === "object" &&
    data !== null &&
    "id" in data &&
    typeof (data as Record<string, unknown>).id === "string"
  );
};

if (isUser(response.data)) {
  const user = response.data;  // User 型として扱える
}
```

例外: `as const`、ライブラリの型定義不足を補う場合のみ、コメント必須。

### 4. 非 null アサーション (`!`) の完全禁止

オプショナルチェーン / null 合体演算子 / 事前ガードを使う。

```typescript
// ❌ NG
const userName = user!.name;

// ✅ OK: オプショナルチェーン + null 合体
const userName = user?.name ?? "Unknown";

// ✅ OK: 事前ガード
if (user) {
  const userName = user.name;
}
```

### 5. 暗黙的 any になるパターン禁止

引数に型を付けない / 初期値のない変数 / コールバック引数を無視 を禁止。

```typescript
// ❌ NG
const calculate = (x, y) => x + y;
let result;
items.map((item) => { /* item の型が推論されない */ });

// ✅ OK
const calculate = (x: number, y: number): number => x + y;
const result: string[] = [];
items.map((item: Item) => item.name);
```

### 6. Index Signature の禁止

`[key: string]: unknown` 系の文字列キーアクセスを禁止。

```typescript
// ❌ NG
type User = { [key: string]: unknown };

// ✅ OK: 定義されたプロパティ
type User = { id: string; name: string };

// ✅ OK: 動的キーが必要なら Record で範囲を絞る
type UserSetting = Record<"theme" | "fontSize", string | number>;
```

### 7. `interface` 禁止

`type` を使う (intersection で継承表現)。

```typescript
// ❌ NG
interface User { id: string; name: string; }

// ✅ OK
type User = { id: string; name: string };

// ✅ OK: 継承
type Admin = User & { permissions: string[] };
```

### 8. `function` 宣言の禁止

アロー関数または関数式を使う。

```typescript
// ❌ NG
function calculateTotal(price: number, qty: number): number {
  return price * qty;
}

// ✅ OK
const calculateTotal = (price: number, qty: number): number => price * qty;

// ✅ OK: 型シグネチャ
type Calculator = (price: number, qty: number) => number;
```

例外: Next.js の `default export function Page(...)` 等、フレームワーク要請の場合のみ可。

## コメント言語

すべてのコード内コメントは**日本語**で記載する。

```typescript
// ❌ NG: Fetch user data from API

// ✅ OK: API からユーザーデータを取得する
```

## エラーハンドリング

`catch` 句の変数は型ガードを実施する。

```typescript
try {
  return await riskyOperation();
} catch (error) {
  if (error instanceof Error) {
    console.error("操作に失敗:", error.message);
    throw new Error(`詳細: ${error.message}`);
  }
  throw new Error("予期しないエラーが発生しました");
}
```

## チェックリスト

- [ ] `any` を使っていない (`unknown` + 型ガード)
- [ ] `as` を使っていない (型ガード関数で代替)
- [ ] `!` (非 null アサーション) を使っていない
- [ ] `interface` を使っていない (`type` のみ)
- [ ] `function` 宣言を使っていない (アロー関数)
- [ ] Index Signature を避けている
- [ ] コメントが日本語
