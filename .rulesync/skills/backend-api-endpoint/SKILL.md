---
name: backend-api-endpoint
description: >-
  バックエンドの API エンドポイント実装ガイド (Go / Kotlin)。"backend-api-endpoint",
  "エンドポイント追加", "API 実装", "CRUD 実装" 等で発動。
targets:
  - "*"
---

# Backend API Endpoint Implementation

## 適用範囲

- Go: `core/`, `examples/go-react/backend/`
- Kotlin: `examples/kotlin-nextjs/backend/`

アーキテクチャ詳細は `/backend-architecture` を参照。

## アーキテクチャ概要

**Package by Feature + クリーンアーキテクチャ + CQRS**

```
依存関係: Presentation → Application → Domain ← Infrastructure
```

| 層             | 責務                                                 |
| -------------- | ---------------------------------------------------- |
| Domain         | エンティティ、リポジトリインターフェース、機能間 DTO |
| Application    | ユースケース、クエリインターフェース (CQRS)          |
| Infrastructure | DB 実装 (Repository = 書き込み、Query = 読み取り)    |
| Presentation   | HTTP ハンドラー、リクエスト/レスポンス変換           |

## 実装順序 (共通)

1. **OpenAPI 仕様定義**
   (Go: `/backend-openapi`、Kotlin: springdoc 等)
2. **Domain 層**: Entity + Repository IF
3. **Application 層**: Query IF + UseCase
4. **Infrastructure 層**: Query 実装 + Repository 実装
5. **Presentation 層**: HTTP ハンドラー
6. **DI 配線**: Go=`server/router.go`、Kotlin=`@Configuration`
7. **認可**: `docs/context/authorization/matrix.md` (正本) と一致させる
8. **テスト**: 各レイヤー (`/backend-testing`)

---

## Go 実装パターン

### 1. OpenAPI 仕様

`/backend-openapi` を参照。`make api-validate && make api-generate` を必ず実行。

### 2. Domain 層

**エンティティ**: `internal/features/<feature>/domain/entity/<feature>.go`

```go
type Client struct {
    id           uuid.UUID
    name         string
    redirectURIs []string
    secretHash   string
    createdAt    time.Time
    updatedAt    time.Time
}

type ClientStatus string

const (
    ClientStatusActive   ClientStatus = "active"
    ClientStatusDisabled ClientStatus = "disabled"
)
```

**リポジトリ IF**: `internal/features/<feature>/domain/repository/<feature>.go`

```go
type ClientRepository interface {
    Create(ctx context.Context, client *entity.Client) (*entity.Client, error)
    Update(ctx context.Context, client *entity.Client) (*entity.Client, error)
    Delete(ctx context.Context, id uuid.UUID) error
    FindByID(ctx context.Context, id uuid.UUID) (*entity.Client, error)
}
```

### 3. Application 層

**Query IF (読取)**: `application/query/<feature>.go`

```go
type ClientQuery interface {
    List(ctx context.Context, limit, offset int) ([]*entity.Client, error)
    Count(ctx context.Context) (int64, error)
}
```

**UseCase**: `application/usecase/<action>_<feature>.go`

```go
type CreateClientInput struct {
    Name         string
    RedirectURIs []string
    SecretHash   string
}

type CreateClientUseCase struct {
    repo   repository.ClientRepository
    logger *slog.Logger
}

func NewCreateClientUseCase(repo repository.ClientRepository, logger *slog.Logger) *CreateClientUseCase {
    return &CreateClientUseCase{repo: repo, logger: logger}
}

func (u *CreateClientUseCase) Execute(ctx context.Context, input CreateClientInput) (*entity.Client, error) {
    client, err := entity.NewClient(uuid.New(), input.Name, input.RedirectURIs, input.SecretHash)
    if err != nil {
        return nil, apperror.BadRequestError(err.Error())
    }
    return u.repo.Create(ctx, client)
}
```

**命名**: 1 UseCase = 1 責務。`{Action}{Feature}UseCase`。

### 4. Infrastructure 層

**Query 実装** (sqlc): `infrastructure/query/<feature>.go`

```go
type clientQuery struct { db *pgxpool.Pool }

func NewClientQuery(db *pgxpool.Pool) query.ClientQuery {
    return &clientQuery{db: db}
}

func (q *clientQuery) List(ctx context.Context, limit, offset int) ([]*entity.Client, error) {
    const maxLimit = 100
    if limit < 1 || limit > maxLimit {
        return nil, fmt.Errorf("limit は 1 から %d の範囲: %d", maxLimit, limit)
    }
    queries := sqlc.New(q.db)
    rows, err := queries.ListClients(ctx, sqlc.ListClientsParams{Limit: int32(limit), Offset: int32(offset)})
    if err != nil {
        return nil, err
    }
    return toDomainClients(rows), nil
}
```

**Repository 実装**: `infrastructure/repository/<feature>.go` (パターンは `/backend-sqlc` 参照)

### 5. Presentation 層

`presentation/handler.go` (oapi-codegen の ServerInterface 実装)

```go
type Handler struct {
    createClientUseCase *usecase.CreateClientUseCase
    clientQuery         query.ClientQuery
}

func (h *Handler) CreateClient(c *gin.Context) {
    var req generated.CreateClientJSONRequestBody
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, apperror.BadRequestError("不正なリクエストです"))
        return
    }
    client, err := h.createClientUseCase.Execute(c.Request.Context(), usecase.CreateClientInput{
        Name:         req.Name,
        RedirectURIs: req.RedirectUris,
        SecretHash:   hashSecret(req.ClientSecret),
    })
    if err != nil {
        respondError(c, err)
        return
    }
    c.JSON(http.StatusCreated, toResponse(client))
}
```

### 6. DI 配線

`internal/server/router.go` で UseCase / Query / Handler を組み立てる。

---

## Kotlin 実装パターン (Spring Boot)

### 1. Domain 層

```kotlin
// domain/entity/Client.kt
class Client private constructor(
    val id: UUID,
    private var name: String,
    val redirectUris: List<String>,
    val secretHash: String,
    val createdAt: Instant,
    val updatedAt: Instant,
) {
    companion object {
        fun new(id: UUID, name: String, redirectUris: List<String>, secretHash: String): Client {
            require(name.isNotBlank()) { "クライアント名は必須です" }
            require(redirectUris.isNotEmpty()) { "redirect_uris は 1 件以上必須です" }
            val now = Instant.now()
            return Client(id, name, redirectUris, secretHash, now, now)
        }
        fun reconstruct(id: UUID, name: String, redirectUris: List<String>, secretHash: String, createdAt: Instant, updatedAt: Instant) =
            Client(id, name, redirectUris, secretHash, createdAt, updatedAt)
    }
    fun name(): String = name
    fun rename(newName: String) {
        require(newName.isNotBlank()) { "クライアント名は必須です" }
        name = newName
    }
}

// domain/repository/ClientRepository.kt
interface ClientRepository {
    fun create(client: Client): Client
    fun findById(id: UUID): Client?
    fun update(client: Client): Client
    fun delete(id: UUID)
}
```

### 2. Application 層

```kotlin
// application/usecase/CreateClientUseCase.kt
@Service
class CreateClientUseCase(
    private val repo: ClientRepository,
) {
    @Transactional
    fun execute(input: CreateClientInput): Client {
        val client = Client.new(UUID.randomUUID(), input.name, input.redirectUris, input.secretHash)
        return repo.create(client)
    }
}

data class CreateClientInput(
    val name: String,
    val redirectUris: List<String>,
    val secretHash: String,
)
```

### 3. Infrastructure 層

```kotlin
// infrastructure/repository/ClientRepositoryImpl.kt
@Repository
class ClientRepositoryImpl(
    private val jdbc: NamedParameterJdbcTemplate,  // または Spring Data JPA
) : ClientRepository {
    override fun create(client: Client): Client { /* ... */ }
    // ...
}
```

### 4. Presentation 層

```kotlin
// presentation/controller/ClientController.kt
@RestController
@RequestMapping("/admin/clients")
class ClientController(
    private val createClientUseCase: CreateClientUseCase,
) {
    @PostMapping
    fun create(@RequestBody @Valid req: CreateClientRequest): ResponseEntity<ClientResponse> {
        val client = createClientUseCase.execute(req.toInput())
        return ResponseEntity.status(HttpStatus.CREATED).body(client.toResponse())
    }
}
```

---

## 認可の実装

`docs/context/authorization/matrix.md` (正本) を参照し、以下を実装する:

- 認証: middleware で OIDC アクセストークン検証 (`/userinfo` 同等)
- 認可: middleware で role / scope claim を検証
- ▲ (条件付き許可): UseCase で `userID == owner` フィルタ

差分検出時は **`.rulesync/rules/authorization-matrix.md` に従い停止しユーザー判断**。

## エラーレスポンス

| 状況               | HTTP | レスポンス                                                     |
| ------------------ | ---- | -------------------------------------------------------------- |
| バリデーション失敗 | 400  | `{"errors": [...]}` または OIDC `{"error": "invalid_request"}` |
| 認証失敗           | 401  | `{"error": "invalid_client"}` (OIDC) など                      |
| 認可拒否           | 403  | `{"errors": [{"code": "forbidden", "message": "..."}]}`        |
| リソース未存在     | 404  | `{"errors": [{"code": "not_found", ...}]}`                     |
| 競合               | 409  | `{"errors": [{"code": "conflict", ...}]}`                      |
| 内部エラー         | 500  | `{"errors": [{"code": "internal", ...}]}`                      |

## チェックリスト

- [ ] OpenAPI 仕様を先に書いた / 更新した
- [ ] Domain / Application / Infrastructure / Presentation を順に実装
- [ ] Domain 層は外側に依存していない
- [ ] 認可マトリクス正本と実装が一致
- [ ] テスト (単体 + 統合) を作成 (`/backend-testing`)
- [ ] 構造化ログを適切に出力 (`/backend-logging`)
- [ ] OIDC エンドポイントは RFC 準拠 (state / nonce / PKCE / redirect_uri)
- [ ] DI 配線済み
- [ ] `make build && make lint && make test` (Go) または `./gradlew check` (Kotlin) がパス
