---
name: frontend-testing
description: >-
  フロントエンドのテスト戦略 (Vitest / Testing Library)。"frontend-testing", "テスト戦略
  (frontend)", "ユニットテスト" 等で発動。
---
# Frontend Testing Guide

## 適用範囲

- `examples/go-react/frontend/` (Vitest + React Testing Library)
- `examples/kotlin-nextjs/frontend/` (Vitest または Jest + Testing Library / Next.js test utilities)

## ツールスタック

| ツール                    | 用途                         |
| ------------------------- | ---------------------------- |
| **Vitest**                | ユニット / 統合テスト        |
| **React Testing Library** | React コンポーネントのテスト |
| **Playwright**            | E2E (`/frontend-e2e` 参照)   |

## テスト実装ポリシー

### 必須 (MUST)

| 対象                                       | 要件                                |
| ------------------------------------------ | ----------------------------------- |
| `src/shared/utils/` 内のユーティリティ関数 | テスト必須、**カバレッジ 100%**     |
| `src/shared/hooks/` 内のカスタムフック     | テスト必須、**カバレッジ 90% 以上** |
| バグ修正時                                 | 再発防止テスト必須                  |

### 推奨 (SHOULD)

| 対象                                       | 要件                     |
| ------------------------------------------ | ------------------------ |
| `src/views/*/hooks/` で 100 行以上のフック | テスト推奨               |
| zod スキーマ (`schemas.ts`)                | バリデーションテスト推奨 |
| 複雑なデータ変換ロジック                   | テスト推奨               |

### 任意 (MAY)

| 対象                       | 要件                |
| -------------------------- | ------------------- |
| 単純なコンポーネント       | 任意                |
| 100 行未満の機能固有フック | 任意                |
| ページレベルコンポーネント | 任意 (E2E でカバー) |

## ファイル命名

- **Vitest テスト**: `.test.ts` / `.test.tsx`

```
src/
├── shared/
│   ├── utils/
│   │   ├── formatDate.ts
│   │   └── formatDate.test.ts     ← 必須
│   └── hooks/
│       ├── useDebounce.ts
│       └── useDebounce.test.ts    ← 必須
└── views/clients/
    ├── hooks/
    │   ├── useClientForm.ts
    │   └── useClientForm.test.ts  ← 推奨 (100 行以上の場合)
    └── schemas.test.ts            ← 推奨
```

## ユーティリティ関数のテスト (必須)

```typescript
// src/shared/utils/formatDate.test.ts
import { describe, it, expect } from "vitest";
import { formatDate } from "@/shared/utils/formatDate";

describe("formatDate", () => {
  it("日付を正しくフォーマットする", () => {
    expect(formatDate(new Date("2024-12-23"))).toBe("2024/12/23");
  });

  it("null の場合は空文字を返す", () => {
    expect(formatDate(null)).toBe("");
  });

  it("undefined の場合は空文字を返す", () => {
    expect(formatDate(undefined)).toBe("");
  });

  it("無効な日付の場合は空文字を返す", () => {
    expect(formatDate(new Date("invalid"))).toBe("");
  });
});
```

## カスタムフックのテスト (必須: shared, 推奨: feature)

```typescript
// src/shared/hooks/useDebounce.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { useDebounce } from "@/shared/hooks/useDebounce";

describe("useDebounce", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("指定した遅延後に値を更新する", async () => {
    const { result, rerender } = renderHook(
      ({ value, delay }) => useDebounce(value, delay),
      { initialProps: { value: "initial", delay: 500 } },
    );

    rerender({ value: "updated", delay: 500 });
    expect(result.current).toBe("initial");

    vi.advanceTimersByTime(500);
    await waitFor(() => expect(result.current).toBe("updated"));
  });
});
```

## zod スキーマのテスト (推奨)

```typescript
// src/views/clients/schemas.test.ts
import { describe, it, expect } from "vitest";
import { clientFormSchema } from "@/views/clients/schemas";

describe("clientFormSchema", () => {
  it("有効なデータでパース成功", () => {
    const valid = {
      name: "App A",
      redirectUris: ["https://app.example.com/cb"],
    };
    expect(clientFormSchema.safeParse(valid).success).toBe(true);
  });

  it("name が空ならバリデーションエラー", () => {
    const invalid = { name: "", redirectUris: ["https://app.example.com/cb"] };
    expect(clientFormSchema.safeParse(invalid).success).toBe(false);
  });

  it("redirect_uris が空配列ならバリデーションエラー", () => {
    const invalid = { name: "App A", redirectUris: [] };
    expect(clientFormSchema.safeParse(invalid).success).toBe(false);
  });
});
```

## テスト命名規約

- テスト名は**日本語**で記載
- 何をテストしているかが明確にわかる

```typescript
// ✅ OK
it("指定した遅延後に値を更新する", () => {
  /* ... */
});
it("null の場合は空文字を返す", () => {
  /* ... */
});

// ❌ NG: 英語
it("should update value after delay", () => {
  /* ... */
});
// ❌ NG: 曖昧
it("正しく動作する", () => {
  /* ... */
});
```

## 必須テストケース (純粋関数)

- **正常系**: 有効データで期待結果
- **異常系**: null / undefined / 空文字 / 無効値
- **境界値**: 最小値 / 最大値 / 空配列

## React Query hook のテスト

生成 hook をモックして、カスタムフックの UI ロジックをテストする。

```typescript
vi.mock("@/api/generated/clients", () => ({
  useGetClients: vi.fn(),
  useCreateClient: vi.fn(),
}));
```

## 実行コマンド (例)

```bash
# React + Vite
npm test
npm run test:coverage

# Next.js (Vitest 採用時)
npm test
```

## 判断プロセス

1. `src/shared/utils/` → テスト**必須** (100%)
2. `src/shared/hooks/` → テスト**必須** (90% 以上)
3. `src/views/*/hooks/` 100 行以上 → テスト**推奨**
4. zod スキーマ → テスト**推奨**
5. その他 → テスト**任意** (E2E でカバー)
