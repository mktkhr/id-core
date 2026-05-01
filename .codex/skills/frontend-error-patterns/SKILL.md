---
name: frontend-error-patterns
description: >-
  フロントエンドのエラーハンドリングパターン (try-catch / Snackbar / API エラー型ガード)。
  "frontend-error-patterns", "エラーハンドリング (frontend)" 等で発動。
---
# Frontend Error Handling

## 適用範囲

- `examples/go-react/frontend/`
- `examples/kotlin-nextjs/frontend/`

## 基本原則

1. すべての非同期処理に `try-catch` を実装し、エラーを包括的に処理
2. catch の変数は `unknown` 型として扱い、型ガードする
3. `console.log` 禁止。`console.error` / `console.warn` のみ
4. **サイレント失敗禁止**: ユーザーに必ずフィードバック (Snackbar / Toast)

## try-catch 基本パターン

```typescript
const fetchData = async (): Promise<Data | null> => {
  try {
    return await riskyOperation();
  } catch (error) {
    if (error instanceof Error) {
      console.error("操作に失敗しました:", error.message);
      throw new Error(`データ取得エラー: ${error.message}`);
    }
    throw new Error("予期しないエラーが発生しました");
  }
};
```

## カスタムフックでのエラー処理

```typescript
const useDataFetch = () => {
  const [data, setData] = useState<Data | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);

  const fetchData = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const result = await getData();
      setData(result);
    } catch (err) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError("予期しないエラーが発生しました");
      }
    } finally {
      setIsLoading(false);
    }
  }, []);

  return { data, error, isLoading, fetchData };
};
```

## ユーザー通知パターン (Snackbar)

ユーザー通知は必ず `@/shared/hooks/useAppSnackbar` (もしくは同等のラッパー hook) を使用する。
notistack ベースのグローバル Provider に接続されており、画面遷移やコンポーネントのアンマウントにも影響されずトーストが残る。

```typescript
import { useAppSnackbar } from "@/shared/hooks/useAppSnackbar";

const MyComponent = () => {
  const { showSuccess, showError, showWarning, showInfo } = useAppSnackbar();

  const handleSave = async () => {
    try {
      await save();
      showSuccess(t("xxx.save.success")); // 右下
    } catch (error) {
      showError(t("common.error.unexpected")); // 上部中央
    }
  };
};
```

### anchorOrigin 自動適用

- `showSuccess` / `showInfo` → 右下
- `showError` / `showWarning` → 上部中央

呼び出し側で anchorOrigin を指定する必要はない。ラッパーが variant ごとに自動付与。

### 禁止パターン

```typescript
// ❌ NG: ローカル state で Snackbar を管理
const [snackbar, setSnackbar] = useState({ isOpen: false, message: "", type: "success" });

// ❌ NG: MUI <Snackbar> / <Alert> を画面に直接配置
<Snackbar open={snackbar.isOpen}>
  <Alert severity="error">{message}</Alert>
</Snackbar>
```

理由: 詳細画面の削除直後に `navigate()` するとローカル Snackbar はアンマウント時に消える。
グローバル Provider 経由なら画面遷移後もトーストが残る。

## API エラーレスポンスの型ガード

```typescript
type ApiError = {
  code: string;
  message: string;
};

const isApiError = (data: unknown): data is ApiError => {
  return (
    typeof data === "object" &&
    data !== null &&
    "code" in data &&
    "message" in data &&
    typeof (data as Record<string, unknown>).code === "string" &&
    typeof (data as Record<string, unknown>).message === "string"
  );
};
```

## OIDC エラーへの対応

OIDC 標準のエラーレスポンス (`{ "error": "...", "error_description": "..." }`) も別途ハンドリング:

```typescript
type OidcError = {
  error: string;
  error_description?: string;
  error_uri?: string;
};

const isOidcError = (data: unknown): data is OidcError => {
  return (
    typeof data === "object" &&
    data !== null &&
    "error" in data &&
    typeof (data as Record<string, unknown>).error === "string"
  );
};

// 表示時はエンドユーザー向けに翻訳する
const oidcErrorToMessage = (e: OidcError): string => {
  switch (e.error) {
    case "invalid_request":
      return t("auth.error.invalidRequest");
    case "invalid_client":
      return t("auth.error.invalidClient");
    case "invalid_grant":
      return t("auth.error.invalidGrant");
    case "access_denied":
      return t("auth.error.accessDenied");
    default:
      return e.error_description ?? t("common.error.unexpected");
  }
};
```

## ログ規約

```typescript
// ❌ NG: console.log は禁止 (Lint で検出)
console.log("エラー:", error);

// ✅ OK: console.error
console.error("API エラー:", error);
```

本番では集約ロギング (Sentry / Datadog 等) に置換する。

## 非推奨パターン

```typescript
// ❌ NG: エラーを無視
try {
  await op();
} catch {
  /* 無視 */
}

// ❌ NG: エラーを隠蔽
try {
  await op();
} catch {
  return null;
}

// ✅ OK: 明示的に null を返すならログを残す
try {
  return await op();
} catch (error) {
  if (error instanceof Error) console.error("操作失敗:", error.message);
  return null;
}
```
