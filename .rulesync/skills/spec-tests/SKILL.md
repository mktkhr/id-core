---
name: spec-tests
description: >-
  API 設計・画面仕様からテスト観点を生成する。"テスト観点生成", "テストケース", "generate-test-cases", "spec-tests", "TDD" 等で発動。
targets:
  - "*"
---

# テスト観点生成スキル

設計書の API 設計・画面仕様からバックエンド (API テスト) とフロントエンド (E2E) のテスト観点を生成する。

## 前提として読み込む context

**必須** (必ず読む):
- 対象の `docs/specs/{N}/index.md` — テスト観点を生成する元

**条件付き** (生成する観点の種類に応じて該当時のみ読む):
- バックエンドテスト観点を生成する場合: `docs/context/testing/backend.md`
- E2E テスト観点を生成する場合: `docs/context/testing/e2e.md` + `docs/context/frontend/registry.md` (data-testid)
- 認可テスト観点に踏み込む場合: `docs/context/authorization/matrix.md`

**読み込まない**: 生成対象でない領域の context は読まない (例: バックエンドテストだけ生成する場合の frontend context)。

詳細は `docs/context/README.md` の対応表を参照。

## 入力

- `$ARGUMENTS` から対象の設計書パスを特定する
- 引数なし: 会話コンテキストから対象設計書を推定する

## バックエンドテスト観点生成ルール

各 API エンドポイントに対して以下のカテゴリでケースを洗い出す:

### 正常系

- 必須フィールドのみでの成功
- オプションフィールド含む成功
- 条件付きフィールドの各パターン
- ページネーション・ソート・フィルターの各パターン

### 異常系

- 必須フィールド欠落 → 400
- 業務キー重複 → 409 + 具体的なエラーコード
- 存在しない ID の参照 → 404 + 具体的なエラーコード
- ドメインバリデーション違反 → 422 + 具体的なエラーコード
- 楽観 / 悲観ロック競合 → 409
- FK 参照ありの削除 → 409

### 認可系

- 各ロールでのアクセス (成功するロール / 403 になるロール)
- ▲ ロールの所有権チェック (自分のデータ → 成功、他人のデータ → 403)

### OIDC OP 固有 (id-core 本体)

- `/authorize` のパラメータバリデーション (`response_type`, `client_id`, `redirect_uri`, `scope`, `state`, `nonce`, `code_challenge`)
- `/token` の認可コード検証・PKCE 検証・clientauth 検証
- `/userinfo` のスコープに基づくクレーム返却
- `/jwks.json` の鍵ローテーション期間中の併存
- ID Token / Access Token の検証 (署名・iss・aud・exp・nbf)

### フォーマット

```markdown
#### #{番号} {メソッド} {パス} ({操作名})

- 正常: {説明} → {HTTP ステータス}
- 異常: {説明} → {HTTP ステータス} {エラーコード}
- 認可: {ロール} → {期待結果}
```

## フロントエンド E2E テスト観点生成ルール

画面単位で以下のカテゴリでシナリオを洗い出す:

### 画面表示

- 初期表示で期待データが表示される
- 条件付き表示 / 非表示が正しく動作する

### ユーザー操作

- フォーム入力 → 保存 → 成功
- 検索 / フィルター → テーブル絞り込み
- ページネーション → ページ切り替え
- ダイアログ表示 → 入力 → 登録

### バリデーション

- 必須項目未入力 → エラー表示
- API エラー (409/422) → エラーメッセージ表示

### 認可

- 権限なしロール → ボタン非表示 / 入力欄読み取り専用

### OIDC RP フロー

- 未ログイン → id-core の `/authorize` にリダイレクト
- code 受領 → token 交換 → ログイン状態へ
- token 期限切れ → refresh token rotation
- ログアウト → セッションクリア

### data-testid 命名規則

```
{画面プレフィックス}-{要素種別}-{名前}
```

**要素種別の略称**:

| 種別 | 略称 | 対象 |
|---|---|---|
| ボタン | `btn` | Button, IconButton |
| テキスト入力 | `input` | TextField |
| セレクト | `select` | Select |
| コンボボックス | `combo` | Autocomplete |
| テーブル | `table` | DataGrid, Table |
| ダイアログ | `dialog` | Dialog |
| リンク | `link` | Link |
| ページネーション | `pager` | Pagination |
| チェック | `check` | Checkbox |
| ラジオ | `radio` | RadioGroup |

**付与基準**: ボタン・フォーム入力・テーブル・ダイアログ・ページネーション操作は MUST。テーブル行・エラー表示は SHOULD。

### フォーマット

```markdown
#### {画面名}

- {カテゴリ}: {説明}
```

## 生成後の検証

- エラーコード一覧の全コードがテスト観点に登場しているか
- 認可マトリクスの全ロール × 全エンドポイントがカバーされているか
- data-testid 一覧の MUST 要素が E2E シナリオで使用されているか
- OIDC OP のセキュリティ観点 (PKCE / state / nonce 検証) が漏れていないか
