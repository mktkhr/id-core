---
name: backend-openapi
description: >-
  Go バックエンド限定の OpenAPI 仕様書編集ガイド (oapi-codegen 前提)。"backend-openapi", "OpenAPI
  編集", "API 仕様書", "エンドポイント追加" 等で発動。
---
# Backend OpenAPI Editor (Go 限定)

## 適用範囲

- `core/` (Go OIDC OP)
- `examples/go-react/backend/` (Go)

> Kotlin (`examples/kotlin-nextjs/backend/`) には **適用しない**
> (springdoc-openapi / コードファースト or yaml ファースト等別の流儀)。

## プロジェクト構成

分割構成の OpenAPI 仕様書を使用:

```
{core or examples/go-react/backend}/api/
├── openapi.yaml              # ルート ($ref 参照のみ)
├── paths/                    # エンドポイント定義
│   └── <機能名>.yaml
└── components/
    ├── schemas/              # スキーマ定義
    │   ├── common.yaml       # Error, HealthResponse
    │   └── <機能名>.yaml
    ├── parameters/
    │   └── common.yaml       # 共通パラメータ
    └── responses/
        └── errors.yaml       # エラーレスポンス
```

## 必須ルール

1. **`required` はユーザーと必ず確認する** (推測で決めない。POST / PATCH / レスポンスそれぞれ確認)
2. **レスポンスは `data` / `errors` ラッパー形式**で統一
3. **全フィールドに `example` 必須** (現実的な値)
4. **全 `description` は日本語**
5. **バリデーション制約をスキーマに明示** (`pattern`, `minLength`, `maxLength`, `enum` など)
6. **OIDC エンドポイントは RFC 準拠**
   - `/authorize`: response_type, client_id, redirect_uri, scope, state, nonce, code_challenge, code_challenge_method
   - `/token`: grant_type, code, redirect_uri, client_id, code_verifier (form-urlencoded)
   - `/userinfo`: Bearer 認証
   - エラーレスポンス: `{"error": "invalid_request", "error_description": "..."}` 形式

## 作業手順

### エンドポイント追加

1. ユーザーに required / optional / バリデーション制約を確認
2. `api/paths/<機能名>.yaml` を作成または編集
3. `api/components/schemas/<機能名>.yaml` にスキーマ追加
4. `api/openapi.yaml` に `$ref` 参照を追加
5. `make api-validate` で検証
6. `make api-generate` でコード生成

### スキーマ追加

1. `api/components/schemas/<機能名>.yaml` に定義追加
2. リクエスト用 (`*Request`) とレスポンス用 (`*Response`) を分離
3. `required` / `nullable` / バリデーション制約を明示

### `$ref` の書き方

- schemas 間: `./filename.yaml#/SchemaName`
- paths から: `../components/schemas/filename.yaml#/SchemaName`
- responses へ: `../components/responses/errors.yaml#/ErrorType`

## スキーマ設計ルール

### 命名

- スキーマ: `UpperCamelCase` (`ClientCreateRequest`, `ClientResponse`)
- パス: `kebab-case` (`/oauth/authorize`, `/oauth/token`, `/admin/clients`)
- パラメータ: `lowerCamelCase` (`clientId`, `redirectUri`)

### required / nullable

- `required`: その**フィールドが存在すること**を要求
- `nullable: true`: 値として `null` を許容
- 両者は独立。`required` かつ `nullable` も成立 (フィールドは存在するが値は null)

### バリデーション

```yaml
type: string
minLength: 1
maxLength: 255
pattern: "^[a-zA-Z0-9_-]+$"
example: "my-client"
```

### enum

```yaml
type: string
enum:
  - authorization_code
  - refresh_token
example: authorization_code
description: OAuth 2.0 の grant_type
```

## レスポンス形式

### 成功 (200 / 201)

```yaml
type: object
required:
  - data
properties:
  data:
    $ref: "../schemas/client.yaml#/Client"
```

### 一覧 (ページネーション)

```yaml
type: object
required:
  - data
  - pagination
properties:
  data:
    type: array
    items:
      $ref: "../schemas/client.yaml#/Client"
  pagination:
    $ref: "../schemas/common.yaml#/Pagination"
```

### エラー (4xx / 5xx)

OIDC エンドポイントは RFC 6749 準拠:

```yaml
type: object
required:
  - error
properties:
  error:
    type: string
    enum: [invalid_request, invalid_client, invalid_grant, unauthorized_client, unsupported_grant_type, invalid_scope]
  error_description:
    type: string
  error_uri:
    type: string
    format: uri
```

非 OIDC API は内部規約 (`{ "errors": [...] }`) で統一する。

## ステータスコード方針

| コード | 用途 |
|---|---|
| 200 | 取得・更新成功 |
| 201 | 作成成功 |
| 204 | 削除成功 / レスポンスボディ不要 |
| 400 | バリデーションエラー / OIDC `invalid_request` |
| 401 | 認証失敗 / OIDC `invalid_client` |
| 403 | 認可拒否 |
| 404 | リソース未存在 |
| 409 | 競合 (重複作成) |
| 500 | サーバー内部エラー |

## コード生成

```bash
make api-validate   # 仕様の検証 (必ず先)
make api-generate   # コード生成
```

生成先: `internal/generated/openapi/` (**直接編集禁止**)
