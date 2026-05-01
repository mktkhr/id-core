---
name: backend-architecture
description: >-
  バックエンドのクリーンアーキテクチャ + Package by Feature ガイド。新機能追加・アーキテクチャ判断時に参照。
  "backend-architecture", "クリーンアーキテクチャ", "DDD" 等で発動。
---
# Backend Architecture Guide

## 適用範囲

- Go: `core/`, `examples/go-react/backend/` (Package by Feature + クリーンアーキテクチャ)
- Kotlin: `examples/kotlin-nextjs/backend/` (Spring Boot + クリーンアーキテクチャ)

## 共通原則

- **クリーンアーキテクチャ**: Domain を中心に据え、外側のレイヤーから内側に依存させる
- **Package by Feature**: 機能ごとにパッケージを切る (`features/<feature_name>/`)
- **CQRS**: 書き込み (Repository) と読み取り (Query) を分離

## レイヤー構造 (Go)

```
internal/features/<feature_name>/
├── application/      # ユースケース層
│   ├── port/         # 外部サービスインターフェース
│   ├── query/        # CQRS 読取インターフェース
│   └── usecase/      # ビジネスロジック
├── domain/           # ドメイン層 (最も内側)
│   ├── entity/       # ドメインエンティティ・Value Object
│   ├── repository/   # リポジトリインターフェース
│   ├── dto/          # 機能間データ受け渡し用 DTO
│   └── service/      # ドメインサービス
├── infrastructure/   # インフラ層
│   ├── external/     # 外部サービス実装
│   ├── query/        # CQRS 読取実装 (sqlc)
│   └── repository/   # 書込実装 (sqlc)
└── presentation/
    └── handler.go    # oapi-codegen ServerInterface 実装
```

## レイヤー構造 (Kotlin / Spring Boot)

```
src/main/kotlin/.../<feature_name>/
├── application/
│   ├── port/         # 外部サービスインターフェース
│   ├── query/        # CQRS 読取インターフェース
│   └── usecase/      # @Service クラス
├── domain/
│   ├── entity/
│   ├── repository/   # interface
│   ├── dto/
│   └── service/
├── infrastructure/
│   ├── external/
│   ├── query/        # @Repository
│   └── repository/   # @Repository
└── presentation/
    └── controller/   # @RestController
```

## 依存ルール

```
Presentation 層 → Application 層 → Domain 層 ← Infrastructure 層
```

### 禁止される依存

1. **Domain 層が外側のレイヤーに依存してはならない**
2. **Presentation 層が Infrastructure 層に直接依存してはならない**
3. **機能パッケージ間の直接依存は禁止**
4. **Infrastructure → Application の参照禁止**
5. **`shared/` パッケージ禁止** (機能間共有を作らない)
6. **features 外から features への参照禁止** (DI を行う `server/` は例外)

## CQRS 適用

| 種別 | インターフェース定義 | 実装 |
|---|---|---|
| 書き込み | `domain/repository/` + `domain/entity/` | `infrastructure/repository/` |
| 読み取り | `application/query/` | `infrastructure/query/` |

## 共通インフラ (Go)

```
internal/
├── apperror/       # カスタムエラー
├── config/         # 設定構造体・ローダー
├── infrastructure/ # DB 接続 (postgres / valkey 等)
├── logger/         # 構造化ログ (slog ベース)
├── middleware/     # HTTP ミドルウェア (request_id / logger / 認可)
├── server/         # ルーター / DI 配線
└── utils/          # ユーティリティ
```

## 機能間の型共有パターン

機能 A が機能 B のデータを利用する場合:

1. **Consumer (A) 側で `domain/dto/` に型を定義**
2. **Consumer (A) 側で `application/query/` にインターフェースを定義** (dto 型を使用)
3. **Provider (B) 側が Consumer の dto 型を import** (Domain 層同士なので許容)
4. **DI 層 (`server/router.go`) で結合**: Provider の実装を Consumer に注入

```
例: rental が item のデータを利用する場合
- rental/domain/dto/        : ItemSummary 型を定義
- rental/application/query/ : ItemQuery インターフェースを定義 (dto 型を使用)
- item/infrastructure/query/: rentaldto.ItemSummary を返すクエリを実装
- server/router.go          : itemQuery を rentalUseCase に注入
```

## 構造体のカプセル化 (Go)

### 原則

構造体は**フィールドを非公開 (小文字)** にし、**コンストラクタで生成**、**ゲッターで読み取り**、**レシーバメソッドで変更**する。

これにより:

- 不正な状態の構造体生成を防止 (コンストラクタでバリデーション)
- フィールドの直接変更を禁止
- ビジネスルールを構造体内に閉じ込める

### コンストラクタのルール

1. **コンストラクタには日本語コメントで「いつ・何のために呼ぶか」を記載**
2. 用途ごとに別のコンストラクタ (新規作成用 / DB 復元用 等)
3. 必須フィールドのバリデーションはコンストラクタ内で行う

### 状態変更のルール

1. **フィールド変更は必ずレシーバメソッド経由**
2. レシーバメソッドにも日本語コメントで「何を変更するか」を記載
3. バリデーションが必要な変更はメソッド内で検証してエラーを返す

### パターン

```go
type Client struct {
    id           uuid.UUID
    name         string
    redirectURIs []string
    secretHash   string
}

// NewClient は OAuth クライアント新規登録時に使用するコンストラクタ
func NewClient(id uuid.UUID, name string, redirectURIs []string, secretHash string) (*Client, error) {
    if name == "" {
        return nil, errors.New("クライアント名は必須です")
    }
    if len(redirectURIs) == 0 {
        return nil, errors.New("redirect_uris は 1 件以上必須です")
    }
    return &Client{id: id, name: name, redirectURIs: redirectURIs, secretHash: secretHash}, nil
}

// ReconstructClient は DB から読み取ったデータで Client を復元するコンストラクタ
func ReconstructClient(id uuid.UUID, name string, redirectURIs []string, secretHash string) *Client {
    return &Client{id: id, name: name, redirectURIs: redirectURIs, secretHash: secretHash}
}

// ゲッター
func (c *Client) ID() uuid.UUID         { return c.id }
func (c *Client) Name() string          { return c.name }
func (c *Client) RedirectURIs() []string { return c.redirectURIs }

// Rename はクライアント名を変更する
func (c *Client) Rename(name string) error {
    if name == "" {
        return errors.New("クライアント名は必須です")
    }
    c.name = name
    return nil
}
```

### 禁止パターン

```go
// ❌ フィールド直接指定で構築
client := &entity.Client{ID: uuid.New(), Name: ""}  // 不正な値が入る

// ✅ コンストラクタ経由
client, err := entity.NewClient(uuid.New(), "App A", []string{"https://app.example.com/cb"}, "hash")

// ❌ フィールドを外部から直接変更
client.Name = "New"

// ✅ レシーバメソッド経由
err := client.Rename("New")
```

## OpenAPI-First (Go)

```bash
# 必ず以下の順序で実行
1. api/openapi.yaml を編集
2. make api-validate
3. make api-generate
4. 実装開始
```

- `internal/generated/openapi/` は**手動編集禁止**
- Request / Response 型は `internal/generated/openapi/types.gen.go` から使用

詳細は `/backend-openapi`, `/backend-sqlc` 参照。

## 新機能追加フロー (Go)

1. `api/openapi.yaml` 定義 → `make api-validate` → `make api-generate`
2. `db/queries/<テーブル>.sql` → `make sqlc-generate`
3. `mkdir -p internal/features/<name>/{application/{query,usecase},domain/{entity,repository},infrastructure/{query,repository},presentation}`
4. Domain (Entity + RepoIF) → Application (QueryIF + UseCase) → Infrastructure (Query / Repo 実装) → Presentation (ServerIF 実装)
5. 各レイヤーでテスト作成
6. `internal/server/router.go` に DI 登録
7. `make test` → `make cover` → `make lint` → `make build`

## 新機能追加フロー (Kotlin / Spring Boot)

1. OpenAPI 仕様 (採用していれば) を更新
2. マイグレーション (Flyway 等) を追加
3. Entity → Repository (interface + 実装) → UseCase (@Service) → Controller (@RestController)
4. 各レイヤーでテスト作成
5. `./gradlew test` → `./gradlew jacocoTestReport` → `./gradlew check`
