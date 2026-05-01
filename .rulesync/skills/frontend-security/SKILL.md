---
name: frontend-security
description: >-
  フロントエンドのセキュリティガイド (XSS / 入力バリデーション / 秘密情報 / URL サニタイズ / OIDC RP)。
  "frontend-security", "フロントセキュリティ", "XSS 対策" 等で発動。
targets:
  - "*"
---

# Frontend Security Guide

フロントエンドはユーザーが直接操作する境界。**入力検証・出力エスケープ・秘密情報管理**を厳格に行う。

## 適用範囲

- `examples/go-react/frontend/`
- `examples/kotlin-nextjs/frontend/`

## 必須セキュリティチェック (コミット前)

- [ ] ハードコードされた秘密情報なし (API キー、トークン)
- [ ] ユーザー入力がバリデーションされている (zod 等)
- [ ] XSS 対策 (`dangerouslySetInnerHTML` を使用していない)
- [ ] 認証・認可フローが正しい (id-core からの ID トークン検証)
- [ ] エラーメッセージに機密データを含めない
- [ ] `localStorage` / `sessionStorage` にトークン全文を保存していない (リフレッシュトークンは特に)
- [ ] `target="_blank"` には `rel="noopener noreferrer"` を付ける

## 秘密情報の管理

```typescript
// ❌ NG: ハードコード
const apiKey = "sk-proj-xxxxx";

// ✅ OK: 環境変数 (Vite: VITE_ プレフィックス、Next.js: NEXT_PUBLIC_ プレフィックス)
const apiKey = import.meta.env.VITE_API_KEY;

if (!apiKey) {
  throw new Error("VITE_API_KEY が設定されていません");
}
```

注意: フロントエンドの環境変数は**ビルド時にバンドルに埋め込まれる** = ブラウザから読める。
真に秘密にすべき値はバックエンド側に置き、API 経由で取得する。

## XSS 対策

```typescript
// ❌ NG
<div dangerouslySetInnerHTML={{ __html: userContent }} />

// ✅ OK: テキストとして表示
<div>{userContent}</div>

// HTML 表示が必要な場合: DOMPurify 等でサニタイズ
import DOMPurify from "dompurify";
<div dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(userContent) }} />
```

## 入力バリデーション (zod)

ユーザー入力は**システム境界で必ずバリデーション**する。

```typescript
import { z } from "zod";

const userSchema = z.object({
  email: z.string().email("有効なメールアドレスを入力してください"),
  age: z.number().int().min(0).max(150),
  name: z.string().min(1, "名前は必須です").max(100),
});

const validateUser = (input: unknown) => {
  const result = userSchema.safeParse(input);
  if (!result.success) {
    throw new Error(result.error.issues[0]?.message ?? "バリデーションエラー");
  }
  return result.data;
};
```

## URL のサニタイズ (open redirect 防止)

```typescript
const isSafeUrl = (url: string): boolean => {
  try {
    const parsed = new URL(url);
    return parsed.protocol === "https:";
  } catch {
    return false;
  }
};

// 使用
{isSafeUrl(userProvidedUrl) && (
  <a href={userProvidedUrl} rel="noopener noreferrer" target="_blank">
    リンク
  </a>
)}
```

## OIDC RP (id-core を OIDC OP として連携する場合)

id-core のサンプルアプリは **OIDC RP** として id-core (OP) と連携する想定。

- **PKCE 必須** (SPA / public client): `code_verifier` を `crypto.getRandomValues` で生成、`code_challenge_method=S256`
- **`state` を生成・セッションストレージに保存・コールバックで照合** (CSRF 対策)
- **`nonce` を生成・ID トークンに含めて検証** (リプレイ攻撃対策)
- **ID トークンの保管**: メモリ上のみが理想。`sessionStorage` 可、`localStorage` は避ける
- **アクセストークン**: 短命 (短い TTL) + リフレッシュトークン rotation
- **リフレッシュトークン**: BFF パターン (HttpOnly Cookie) を推奨。ブラウザに置く場合は `localStorage` 禁止
- **ID トークンをコンソールに出さない**

## CSRF 対策

- API リクエストに CSRF token (DOM の meta tag から読む or Cookie + Header の two-token pattern)
- consent / authorize 画面を含む state-changing リクエストは特に注意

## CSP (Content Security Policy)

- `default-src 'self'`
- `script-src` に `'unsafe-inline'` を含めない (含む場合は nonce / hash)
- `frame-ancestors 'none'` (clickjacking 対策)

## セキュリティ対応プロトコル

セキュリティ問題が発見された場合:

1. 直ちに作業を停止
2. CRITICAL を先に修正
3. 漏洩した秘密情報をローテーション
4. コードベース全体で類似問題をスキャン
5. ADR (`docs/adr/`) に記録
