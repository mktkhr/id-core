---
name: frontend-form-patterns
description: >-
  react-hook-form + zod でのフォーム実装パターン。"frontend-form-patterns",
  "フォーム実装", "バリデーション", "react-hook-form" 等で発動。
targets:
  - "*"
---

# Frontend フォームパターン

react-hook-form + zod でのフォーム実装パターン。MUI / shadcn 等 UI ライブラリ非依存の部分。

## 適用範囲

- `examples/go-react/frontend/` (MUI 想定 → `Controller` パターン)
- `examples/kotlin-nextjs/frontend/` (shadcn / 任意の UI ライブラリ)

## 基本構成

```
src/views/[feature]/
├── hooks/
│   └── useClientForm.ts     # フォームロジック (react-hook-form + zod)
├── components/
│   └── ClientForm.tsx       # フォーム UI
├── schemas.ts               # zod スキーマ (UI 固有バリデーション)
└── types.ts                 # UI 固有型定義
```

## zod スキーマ定義

API 側のバリデーション (文字数制限等) は Orval が自動生成する zod スキーマを利用できる。
`schemas.ts` には**UI 固有のバリデーション**のみを定義する。

```typescript
// src/views/clients/schemas.ts
import { z } from "zod";

export const clientFormSchema = z.object({
  name: z.string().min(1, "クライアント名は必須です").max(255),
  redirectUris: z
    .array(z.string().url("有効な URL を入力してください"))
    .min(1, "1 件以上必要です"),
  description: z
    .string()
    .max(1000, "1000 文字以内で入力してください")
    .optional(),
});

export type ClientFormData = z.infer<typeof clientFormSchema>;
```

### スキーマ設計ルール

- バリデーションメッセージは**日本語**
- `z.infer<typeof schema>` で型を導出 (型とスキーマの二重定義を避ける)
- API 型 (Orval / 共通生成) とフォーム型は異なる場合がある。変換はフック内で行う

## カスタムフック (例: react-hook-form + Orval)

```typescript
// src/views/clients/hooks/useClientForm.ts
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useQueryClient } from "@tanstack/react-query";
import {
  useCreateClient,
  getGetClientsQueryKey,
} from "@/api/generated/clients";
import { useAppSnackbar } from "@/shared/hooks/useAppSnackbar";
import { clientFormSchema, type ClientFormData } from "../schemas";

export const useClientForm = () => {
  const queryClient = useQueryClient();
  const { showSuccess, showError } = useAppSnackbar();

  const form = useForm<ClientFormData>({
    resolver: zodResolver(clientFormSchema),
    defaultValues: { name: "", redirectUris: [""], description: undefined },
  });

  const { mutate: createClient, isPending } = useCreateClient({
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: getGetClientsQueryKey() });
      showSuccess(t("clients.create.success"));
      form.reset();
    },
    onError: () => {
      showError(t("clients.create.error"));
    },
  });

  const handleSubmit = form.handleSubmit((data) => {
    createClient({
      data: {
        name: data.name,
        redirect_uris: data.redirectUris,
        description: data.description,
      },
    });
  });

  return { form, handleSubmit, isPending };
};
```

## MUI Controller パターン (`go-react/frontend`)

MUI の TextField 等は uncontrolled が弱いため、`Controller` で明示的に接続する。

```typescript
import { Controller } from "react-hook-form";
import { TextField, Button, Stack } from "@mui/material";
import { useClientForm } from "../hooks/useClientForm";

export const ClientForm = () => {
  const { form, handleSubmit, isPending } = useClientForm();
  const { control, formState: { errors } } = form;

  return (
    <Stack component="form" onSubmit={handleSubmit} spacing={2}>
      <Controller
        name="name"
        control={control}
        render={({ field }) => (
          <TextField
            {...field}
            label={t("clients.form.name")}
            error={!!errors.name}
            helperText={errors.name?.message}
            fullWidth
          />
        )}
      />
      <Button type="submit" variant="contained" disabled={isPending}>
        {t("clients.form.submit")}
      </Button>
    </Stack>
  );
};
```

## バリデーション戦略

### 層の分離

| 層                     | 責務                 | 実装                                 |
| ---------------------- | -------------------- | ------------------------------------ |
| UI バリデーション      | フォーム入力チェック | zod + react-hook-form (`schemas.ts`) |
| API バリデーション     | リクエストスキーマ   | Orval / OpenAPI 生成 zod             |
| サーバーバリデーション | ビジネスルール       | バックエンド API                     |

### タイミング

- **送信時** (`onSubmit`): zod スキーマで一括バリデーション
- **blur 時のバリデーションは行わない** (UX 優先)
- サーバーエラーは Snackbar で表示 + 必要なら `setError` でフィールド反映

### サーバーエラーをフィールドに反映

```typescript
const { mutate: createClient } = useCreateClient({
  onError: (error) => {
    if (isApiValidationError(error)) {
      error.errors.forEach(({ field, message }) => {
        form.setError(field as keyof ClientFormData, { message });
      });
      return;
    }
    showError(t("common.error.unexpected"));
  },
});
```

## 編集フォームパターン

```typescript
export const useClientEditForm = (clientId: string) => {
  const { data: client } = useGetClient(clientId);

  const form = useForm<ClientFormData>({
    resolver: zodResolver(clientFormSchema),
  });

  useEffect(() => {
    if (client) {
      form.reset({
        name: client.name,
        redirectUris: client.redirect_uris,
        description: client.description ?? undefined,
      });
    }
  }, [client, form]);

  // ... mutation 部分
};
```

## 禁止パターン

```typescript
// ❌ NG: useState + 手動バリデーション
const [name, setName] = useState("");
const [errors, setErrors] = useState({});
const validate = () => {
  if (!name) setErrors({ name: "必須" });
};

// ✅ OK: react-hook-form + zod
const form = useForm({ resolver: zodResolver(schema) });

// ❌ NG: フォーム状態をコンポーネントに直接
const ClientForm = () => {
  const [formData, setFormData] = useState({ name: "" });
  // UI とロジック混在
};

// ✅ OK: ロジックはフック化
const ClientForm = () => {
  const { form, handleSubmit } = useClientForm();
  // UI のみ
};
```
