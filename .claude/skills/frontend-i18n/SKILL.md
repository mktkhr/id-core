---
name: frontend-i18n
description: 'フロントエンドの国際化 (i18n) パターン。"frontend-i18n", "国際化", "i18n", "ja/en 同時更新" 等で発動。'
---
# Frontend i18n パターン

## 適用範囲

- `examples/go-react/frontend/` (react-i18next 想定)
- `examples/kotlin-nextjs/frontend/` (next-intl / i18next 等を採用した場合)

## 基本方針

- **デフォルト言語**: `ja`
- **フォールバック**: `en`
- **配置**: `src/locales/` (Next.js: `app/[locale]/messages/` 等)
- **アクセス**: `t("feature.section.key")`

## 翻訳キー更新ルール (CRITICAL)

翻訳キーを追加・変更する際は、**`ja.json` と `en.json` を同時に更新する**。

- 片方のみの更新は禁止
- 両方で同じキー構造を維持

```json
// ❌ NG: en に追加忘れ
// ja.json
{ "user": { "name": "名前", "newField": "新規項目" } }
// en.json
{ "user": { "name": "Name" } }

// ✅ OK: 両方更新
// ja.json
{ "user": { "name": "名前", "newField": "新規項目" } }
// en.json
{ "user": { "name": "Name", "newField": "New Field" } }
```

## キー命名規約

機能ベースの階層構造:

```json
{
  "featureName": {
    "section": {
      "key": "翻訳テキスト"
    }
  }
}
```

例:

```json
{
  "auth": {
    "login": {
      "title": "ログイン",
      "submitButton": "ログインする",
      "errorInvalidCredentials": "メールアドレスまたはパスワードが間違っています"
    },
    "consent": {
      "title": "アクセス許可",
      "approveButton": "許可する",
      "denyButton": "拒否する"
    }
  }
}
```

## 使用パターン (react-i18next)

```typescript
import { useTranslation } from "react-i18next";

const LoginPage = () => {
  const { t } = useTranslation();
  return (
    <div>
      <h1>{t("auth.login.title")}</h1>
      <button>{t("auth.login.submitButton")}</button>
    </div>
  );
};
```

## 動的な翻訳キー

動的キーの構築は基本避ける。やむを得ない場合は型安全に。

```typescript
// ❌ NG: 動的キー (型安全でない)
const key = `status.${status}`;
t(key);

// ✅ OK: 静的キーマッピング
const STATUS_KEYS = {
  pending: "status.pending",
  active: "status.active",
  expired: "status.expired",
} as const;

t(STATUS_KEYS[status]);
```

## ハードコード禁止

UI に表示するテキストはすべて i18n を経由する。

```typescript
// ❌ NG
<Typography>ログイン</Typography>

// ✅ OK
<Typography>{t("auth.login.title")}</Typography>
```

## ディレクトリ構成 (例)

```
src/locales/
├── ja.json    # 日本語 (デフォルト)
└── en.json    # 英語 (フォールバック)
```

## チェックリスト (PR レビュー時)

- [ ] `ja.json` と `en.json` の両方を更新した
- [ ] キー構造が両方で一致している
- [ ] UI テキストをハードコードしていない
- [ ] 動的キー構築を避けた (使う場合は静的マッピング)
