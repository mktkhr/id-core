---
name: frontend-mui-patterns
description: >-
  Material-UI (MUI) 使用規約 (sx / テーマトークン / Snackbar / Dialog / TextField /
  DatePicker)。 "frontend-mui-patterns", "MUI", "sx" 等で発動。
---
# Frontend MUI (Material-UI) パターン

## 適用範囲

**`examples/go-react/frontend/` (MUI 使用) のみ**

> Next.js 側 (`examples/kotlin-nextjs/frontend/`) は MUI を使わない場合あり。
> 採用時は同じ規約に従ってよいが、Next.js App Router での MUI セットアップ (Server Component との互換) が別途必要。

## 基本方針

- **MUI v7+** + **Emotion** (CSS-in-JS、`sx` プロパティ優先)
- **@mui/x-date-pickers** + date-fns adapter (日付ピッカー)
- date-fns ロケールは `ja`

## スタイリング規約

### `sx` プロパティを優先

```typescript
// ✅ 推奨
<Box sx={{ display: "flex", gap: 2, mt: 3 }}>
  <Typography sx={{ fontWeight: "bold" }}>タイトル</Typography>
</Box>

// ❌ 非推奨: インライン style
<Box style={{ display: "flex", gap: 16, marginTop: 24 }}>
```

### テーマトークンを使用

ハードコードされたカラーコード・数値を避ける。

```typescript
// ❌ NG
<Box sx={{ color: "#1976d2", padding: "16px" }} />

// ✅ OK
<Box sx={{ color: "primary.main", p: 2 }} />
```

## Snackbar / Alert (重要)

**MUI の `Snackbar` / `Alert` を直接使用するのは禁止。**
必ず共通フック `useAppSnackbar` (`@/shared/hooks/useAppSnackbar`) 経由で通知を出す。
ベースは notistack、`App.tsx` の `SnackbarProvider` に接続済み。

```typescript
import { useAppSnackbar } from "@/shared/hooks/useAppSnackbar";

const MyComponent = () => {
  const { showSuccess, showError, showWarning, showInfo } = useAppSnackbar();

  const handleSave = async () => {
    try {
      await save();
      showSuccess(t("xxx.save.success")); // 右下
    } catch {
      showError(t("common.error.unexpected")); // 上部中央
    }
  };
};
```

- severity ごとの anchorOrigin (`error`/`warning` → 上部中央、`success`/`info` → 右下) はラッパーが自動適用
- Provider がルート直下にあるため、コンポーネントがアンマウントされてもトーストは残る (詳細画面で削除→navigate する場合など)
- **画面で `useState<SnackbarOption>` を持ったり、`<Snackbar>` / `<Alert>` の JSX を書いたりしない**

詳細は `/frontend-error-patterns` 参照。

## Dialog

```typescript
type DialogState = { isOpen: boolean; targetId: string | null };

const useConfirmDialog = () => {
  const [dialog, setDialog] = useState<DialogState>({
    isOpen: false,
    targetId: null,
  });
  const openDialog = useCallback(
    (id: string) => setDialog({ isOpen: true, targetId: id }),
    [],
  );
  const closeDialog = useCallback(
    () => setDialog({ isOpen: false, targetId: null }),
    [],
  );
  return { dialog, openDialog, closeDialog };
};
```

## TextField (react-hook-form 連携)

`Controller` パターンで接続 (詳細は `/frontend-form-patterns`)。

```typescript
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
```

## DatePicker (date-fns + ja)

```typescript
import { LocalizationProvider } from "@mui/x-date-pickers/LocalizationProvider";
import { AdapterDateFns } from "@mui/x-date-pickers/AdapterDateFns";
import { DatePicker } from "@mui/x-date-pickers/DatePicker";
import { ja } from "date-fns/locale";

// Provider (App.tsx)
<LocalizationProvider dateAdapter={AdapterDateFns} adapterLocale={ja}>
  {children}
</LocalizationProvider>

// react-hook-form 連携
<Controller
  name="expiresAt"
  control={control}
  render={({ field }) => (
    <DatePicker
      label={t("clients.form.expiresAt")}
      value={field.value}
      onChange={field.onChange}
      slotProps={{
        textField: {
          error: !!errors.expiresAt,
          helperText: errors.expiresAt?.message,
          fullWidth: true,
        },
      }}
    />
  )}
/>
```

## Tooltip

```typescript
<Tooltip title={t("clients.detail.tooltip")}>
  <IconButton><InfoIcon /></IconButton>
</Tooltip>
```

## Loading 状態

```typescript
{isLoading && (
  <Box sx={{
    position: "absolute",
    inset: 0,
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    bgcolor: "rgba(255, 255, 255, 0.7)",
    zIndex: 1,
  }}>
    <CircularProgress />
  </Box>
)}
```

## ビューポート単位の使用制限

`vh` / `dvh` / `svh` / `lvh` は**レイアウトコンポーネント (`src/layouts/`) でのみ使用可能**。
それ以外での使用は禁止。

理由: 子コンポーネントがビューポート単位でサイズ指定すると、親要素を突き破ってレイアウトが崩れる。
子は `flex: 1`、`height: '100%'` 等で親に追従させる。

```typescript
// ❌ NG: 子で vh
const ClientList = () => (
  <Box sx={{ height: "100vh" }}>...</Box>
);

// ✅ OK: 親の flex に従う
const ClientList = () => (
  <Box sx={{ flex: 1, overflow: "auto" }}>...</Box>
);

// ✅ OK: layout のみ
// src/layouts/AppLayout.tsx
<Box sx={{ height: "100dvh", display: "flex", flexDirection: "column" }}>
```

## 禁止パターン

- `makeStyles` / `withStyles` 使用禁止 (`sx` を使う)
- インライン `style` オブジェクト適用禁止
- ハードコードされたカラーコード・サイズ値禁止
- テーマカラーを文字列で直接指定しない (`#1976d2` → `primary.main`)
- `vh` / `dvh` / `svh` / `lvh` を `src/layouts/` 以外で使用しない
- `<Snackbar>` / `<Alert>` の直接配置禁止 (`useAppSnackbar` を使う)
