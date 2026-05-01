---
name: backend-sqlc
description: >-
  Go バックエンド限定の sqlc / Repository / Query 実装ガイド。"backend-sqlc", "sqlc 使う",
  "Repository 実装" 等で発動。
---
# Backend SQLC Guide (Go 限定)

## 適用範囲

- `core/` (Go OIDC OP)
- `examples/go-react/backend/` (Go)

> Kotlin (`examples/kotlin-nextjs/backend/`) には **適用しない** (Spring Data JPA / jOOQ / Exposed 等を使う)。

## 概要

sqlc は SQL クエリから Go コードを自動生成する。型安全な DB 操作を実現する。

## クエリファイル

配置: `db/queries/<テーブル名>.sql`

### フォーマットルール

- カラムは**1 行 1 項目**で記述する (差分が見やすい、保守しやすい)
- VALUES / SET / RETURNING も同様
- WHERE 条件が複数ある場合は AND/OR ごとに改行
- LIMIT / OFFSET は別行
- 各クエリ冒頭に**日本語の 1 行コメント**で目的を記載する

```sql
-- ✅ 良い例
-- name: GetClientByID :one
-- 指定 ID の OAuth クライアントを取得する
SELECT
    id,
    name,
    redirect_uris,
    client_secret_hash,
    created_at,
    updated_at
FROM clients
WHERE id = $1;

-- name: CreateClient :one
-- OAuth クライアントを新規作成する
INSERT INTO clients (
    name,
    redirect_uris,
    client_secret_hash
) VALUES (
    $1,
    $2,
    $3
)
RETURNING
    id,
    name,
    redirect_uris,
    client_secret_hash,
    created_at,
    updated_at;
```

### 悪い例

```sql
-- ❌ 1 行に詰める (差分追跡不能)
SELECT id, name, redirect_uris FROM clients WHERE id = $1;
```

## 生成

- 生成先: `internal/generated/sqlc/` (**手動編集禁止**)
- コマンド: `make sqlc-generate` (db/queries/*.sql から Go コード生成)

## Repository 実装パターン (書き込み系)

```go
type repositoryImpl struct {
    queries *sqlc.Queries
}

func NewRepositoryImpl(db *pgxpool.Pool) repository.ClientRepository {
    return &repositoryImpl{queries: sqlc.New(db)}
}

func (r *repositoryImpl) Create(ctx context.Context, client *entity.Client) (*entity.Client, error) {
    result, err := r.queries.CreateClient(ctx, sqlc.CreateClientParams{
        Name:             client.Name(),
        RedirectUris:     client.RedirectURIs(),
        ClientSecretHash: client.ClientSecretHash(),
    })
    if err != nil {
        logger.LogDatabaseError(ctx, "CREATE", "clients", err, "name", client.Name())
        return nil, apperror.InternalServerError("クライアントの作成に失敗しました")
    }
    return toDomainClient(result), nil
}
```

## Query 実装パターン (CQRS 読み取り系)

```go
type queryImpl struct {
    queries *sqlc.Queries
}

func (q *queryImpl) ListClients(ctx context.Context, param query.ListClientsParam) ([]query.ClientListResult, error) {
    rows, err := q.queries.ListClients(ctx, sqlc.ListClientsParams{
        Limit:  param.Limit,
        Offset: param.Offset,
    })
    if err != nil {
        return nil, err
    }
    return toClientListResults(rows), nil
}
```

## ドメインエンティティ変換

```go
func toDomainClient(row sqlc.Client) *entity.Client {
    return entity.ReconstructClient(
        row.ID,
        row.Name,
        row.RedirectUris,
        row.ClientSecretHash,
        row.CreatedAt.Time,
        row.UpdatedAt.Time,
    )
}
```

## マイグレーション

- CLI: `make migrate-*` 系コマンドで管理 (golang-migrate / atlas など)
- 命名規則: `NNNNNN_動詞_対象.sql` (例: `000001_create_clients.sql`)
- `up.sql` と `down.sql` は**必ずペアで作成**

### 外部キー制約 (必須チェック)

`REFERENCES` を使う場合:

1. **`ON DELETE` を必ず明示指定** (省略禁止。デフォルト `NO ACTION` だと参照先削除でエラー)
2. **参照先テーブルにソフトデリート (`is_active` 等) がある場合**、当該 FK の振る舞いをマイグレーションコメントに明記する

### ソフトデリートとクエリの安全性

`is_active` カラムを持つテーブルへの SELECT:

1. **すべての SELECT で無効レコードの除外を検討すること** — `WHERE is_active = true` が必要か、意図的に含めるかをコメントに明記
2. **JOIN で `is_active` テーブルを参照する場合も同様**
3. **意図的に含める場合**は理由をコメント (例: `-- 失効済みも含めて履歴表示するため is_active フィルタなし`)

## DB 操作ルール

- **CUD**: Domain Entity を介して Repository に渡す
- **SELECT (Repository)**: 全カラム取得 (Entity 復元のため)
- **SELECT (Query)**: 必要カラムのみで可 (sqlc クエリで列挙)

## エラーハンドリング

```go
result, err := r.queries.CreateClient(ctx, params)
if err != nil {
    logger.LogDatabaseError(ctx, "CREATE", "clients", err, "name", name)
    return nil, apperror.InternalServerError("クライアントの作成に失敗しました")
}
```

## OIDC 特有のテーブル設計の留意点

- `auth_codes`, `refresh_tokens` 等の TTL カラムは `expires_at TIMESTAMPTZ NOT NULL` (UTC) で持つ
- 認可コード再利用検知用に `consumed_at` を持ち、UPDATE で 1 度限り使用を担保 (`WHERE consumed_at IS NULL`)
- リフレッシュトークン rotation は `parent_id` または `family_id` を持ち、family 単位で失効可能にする
