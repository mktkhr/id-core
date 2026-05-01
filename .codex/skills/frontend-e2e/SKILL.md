---
name: frontend-e2e
description: >-
  Playwright E2E テスト + data-testid 命名規則。"frontend-e2e", "E2E テスト", "Playwright",
  "data-testid" 等で発動。
---
# Frontend E2E テスト (Playwright)

## 適用範囲

- `examples/go-react/frontend/`
- `examples/kotlin-nextjs/frontend/`

## data-testid 命名規則

すべてのユーザー操作可能な UI 要素に `data-testid` を付与する。

### フォーマット

```
{画面}-{要素種別}-{名前}
```

### 画面プレフィックス例 (id-core サンプル想定)

| 画面                       | プレフィックス   |
| -------------------------- | ---------------- |
| クライアント一覧           | `clients-list`   |
| クライアント詳細           | `clients-detail` |
| クライアント作成ダイアログ | `clients-create` |
| ログイン                   | `auth-login`     |
| 同意 (consent)             | `auth-consent`   |
| ユーザー詳細               | `users-detail`   |

### 要素種別

| 種別             | 略称     | 対象                     |
| ---------------- | -------- | ------------------------ |
| ボタン           | `btn`    | Button, IconButton       |
| テキスト入力     | `input`  | TextField                |
| セレクト         | `select` | Select                   |
| コンボボックス   | `combo`  | Autocomplete (freeSolo)  |
| テーブル         | `table`  | DataGrid, Table          |
| カード           | `card`   | Card                     |
| リンク           | `link`   | Link, テーブル行クリック |
| ダイアログ       | `dialog` | Dialog                   |
| チップ           | `chip`   | Chip                     |
| ラジオ           | `radio`  | RadioGroup               |
| チェック         | `check`  | Checkbox                 |
| ファイル         | `file`   | ファイルアップロード     |
| ページネーション | `pager`  | Pagination               |

### 使用例

```tsx
// ボタン
<Button data-testid="clients-list-btn-add">クライアントを追加</Button>
<Button data-testid="clients-detail-btn-save-basic">保存</Button>

// 入力欄
<TextField data-testid="clients-create-input-name" />
<TextField data-testid="clients-detail-input-redirect-uri" />

// コンボボックス
<Autocomplete data-testid="clients-create-combo-grant-types" />

// セレクト
<Select data-testid="clients-detail-select-status" />

// テーブル
<DataGrid data-testid="clients-list-table-main" />

// ダイアログ
<Dialog data-testid="clients-create-dialog" />

// ページネーション
<Select data-testid="clients-list-pager-size" />
<Button data-testid="clients-list-pager-prev" />
<Button data-testid="clients-list-pager-next" />
```

## Playwright での使用

```typescript
await page.getByTestId("clients-list-btn-add").click();
await page.getByTestId("clients-create-input-name").fill("App A");
await page.getByTestId("clients-create-dialog").waitFor({ state: "visible" });
```

## 安定化ガイド (Flake 対策)

### セレクタ方針

- `getByRole({ name })` は文言変更や再描画に弱いため、操作対象は **`getByTestId` を優先**
- 文言依存セレクタを使う場合は、`dialog` や `table` などのスコープを先に限定

### 同期方針

- **操作前**: `expect(locator).toBeVisible()` / `toBeEnabled()` を確認してから `click` / `fill`
- **操作後**: API 連動 UI は `waitForRequest` + `waitForResponse` で通信完了を待つ
- **ダイアログ**: 開く/閉じるごとに `toBeVisible` / `toBeHidden` を明示

### モック方針

- 各テストで利用するエンドポイントを漏れなく `page.route` で捕捉
- 代表例: 認証 (`/.well-known/openid-configuration`, `/userinfo`), 一覧取得, 詳細取得, 作成/更新/削除
- クエリ付き URL はワイルドカード使用、未捕捉リクエストを発生させない

### 並列実行方針

- 共有状態が原因で不安定な spec は一時的に `describe.configure({ mode: "serial" })` で安定化を確認
- 安定化後は状態分離 (テストデータ・モック・待機) を入れて並列実行へ戻す
- 対象 spec を連続複数回実行して全通してから全体実行に戻す

## OIDC フロー特有のテスト観点

id-core サンプルアプリ (RP) の E2E では特に以下を検証:

- ログイン → id-core への redirect → 認証完了 → コールバック → アプリ画面への戻り
- consent 画面の同意 / 拒否
- アクセストークン期限切れ → リフレッシュトークン rotation
- 不正な `state` / `nonce` のエラー検出
- redirect_uri 不一致のエラー画面

モックの場合は id-core OP のレスポンスを `page.route` で固定する。

## E2E 実行 (例)

```bash
cd examples/go-react/frontend
npx playwright test               # 全実行
npx playwright test e2e/login.spec.ts  # 単体実行
npx playwright test --headed      # ヘッドあり (デバッグ)
npx playwright test --ui          # UI モード
```

`make e2e` 等のラッパーターゲットを採用する場合は、guard / typecheck / install を順に走らせる。

## data-testid 付与の判断基準

### 必須 (MUST)

- ボタン (保存・キャンセル・追加・削除等の操作系)
- フォーム入力欄 (テキスト、セレクト、コンボボックス、ラジオ、チェックボックス)
- テーブル本体
- ダイアログ
- ナビゲーションリンク
- ページネーション操作

### 推奨 (SHOULD)

- テーブル行 (`clients-list-link-row-{id}` 等、動的 ID 付き)
- エラーメッセージ表示領域
- ローディング表示

### 不要 (NOT NEEDED)

- 装飾的な要素 (アイコン、ディバイダー)
- 静的テキスト (見出し、ラベル)
