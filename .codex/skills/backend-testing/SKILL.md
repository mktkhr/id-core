---
name: backend-testing
description: 'バックエンドのテスト戦略・パターンガイド (Go / Kotlin)。"backend-testing", "テスト書く", "TDD" 等で発動。'
---
# Backend Testing Guide

## 適用範囲

- Go: `core/`, `examples/go-react/backend/`
- Kotlin: `examples/kotlin-nextjs/backend/`

## カバレッジ要件 (目安)

| レイヤー               | 目標 |
| ---------------------- | ---- |
| Domain 層              | 100% |
| 重要 BL (UseCase コア) | 100% |
| UseCase 層             | 95%  |
| Presentation 層        | 90%  |

## TDD フロー (必須)

1. テストケース設計 → テスト作成 (RED)
2. 最小限の実装 (GREEN)
3. リファクタリング (IMPROVE)
4. カバレッジ確認

## Go (`core/` / `examples/go-react/backend/`)

### フレームワーク

- **testify/suite**: テストスイート
- **testify/mock**: モック
- **httptest**: HTTP テスト
- 競合状態検出: `go test -race`

### ファイル構成

```
foo_usecase.go                       # 実装
foo_usecase_test.go                  # 単体テスト
foo_repository_integration_test.go   # 統合テスト (DB 必須)
mock_foo_repository_test.go          # モック
```

### 命名規則

```
Test{メソッド}_{種類}_{詳細}
```

例:

- `TestCreateUser_Success`
- `TestCreateUser_ValidationError_EmptyName`
- `TestRefreshToken_BoundaryValue_Expired`
- `TestAuthorize_BusinessLogicError_InvalidRedirectURI`

すべてのテストに**日本語で「何をテストしているか」のコメント**を記載する。

### 必須テストケース

- **正常系**: 有効データで成功
- **異常系**: 必須欠如 / 文字数違反 / 範囲外 / フォーマット違反 / 不正 JSON
- **境界値**: 最小値 / 最大値 / 最小値-1 / 最大値+1
- **OIDC 特有**: 期限切れ token, 不一致 state/nonce, 1 度使用済みコード, redirect_uri 不一致

### バリデーション責務

| 層           | 責務                              |
| ------------ | --------------------------------- |
| Presentation | フィールド (形式・文字数・必須)   |
| UseCase      | ビジネスルール (重複・存在・権限) |

### AAA パターン

```go
func (suite *UsecaseTestSuite) TestCreateClient_Success() {
    // クライアントを正常に登録できることを確認する
    // Arrange
    input := usecase.CreateClientInput{Name: "App A", RedirectURIs: []string{"https://app.example.com/cb"}}
    suite.mockRepo.On("Create", mock.Anything, mock.Anything).
        Return(&entity.Client{}, nil)

    // Act
    result, err := suite.usecase.CreateClient(suite.ctx, input)

    // Assert
    assert.NoError(suite.T(), err)
    assert.Equal(suite.T(), "App A", result.Name())
}
```

### テーブル駆動テスト

```go
tests := []struct {
    name    string
    input   string
    wantErr bool
}{
    {"正常系: 有効な redirect_uri", "https://app.example.com/cb", false},
    {"異常系: HTTP 平文", "http://app.example.com/cb", true},
    {"異常系: 空", "", true},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { /* ... */ })
}
```

## 統合テスト (Go)

- テスト用 DB は**マイグレーション済み** (DDL 直接実行 / DROP TABLE 禁止)
- 接続: 環境変数 `TEST_DATABASE_URL` で制御 (フォールバック DSN 禁止、ポートハードコード禁止)
- 未設定時はスキップ (FAIL ではない)
- 各テスト前に `TRUNCATE ... CASCADE` でクリーンアップ
- `make cover` 系のターゲットでマイグレーション適用 + テスト実行を一括化

## 品質ルール

- `require.NoError(t, err)` パターン (`_` でのエラー無視禁止)
- `TestMain` 内のフェイル: `log.Fatalf` (`fmt.Printf` で済まさない)
- エラーメッセージは日本語で具体的に (何が失敗したかが分かる)

## Kotlin (`examples/kotlin-nextjs/backend/`)

### フレームワーク

- **JUnit 5** + **AssertJ** (アサーション)
- **MockK** (モック)
- **Spring Boot Test** + **Testcontainers** (統合テスト)
- カバレッジ: **JaCoCo**

### ファイル構成

```
src/main/kotlin/.../FooUseCase.kt
src/test/kotlin/.../FooUseCaseTest.kt           # 単体
src/test/kotlin/.../FooRepositoryIntegrationTest.kt  # 統合
```

### 命名規則 (BDD 形式)

```kotlin
@Test
fun `クライアントを正常に登録できる`() { /* ... */ }

@Test
fun `redirect_uri が空のとき BadRequest を返す`() { /* ... */ }
```

### Testcontainers パターン

```kotlin
@Testcontainers
@SpringBootTest
class ClientRepositoryIntegrationTest {
    companion object {
        @Container
        val postgres = PostgreSQLContainer("postgres:16-alpine")
    }
    // ...
}
```

## 実行コマンド (例)

### Go

```bash
make -C core test              # 全テスト
make -C core test-unit         # 単体のみ (-short)
make -C core test-integration  # 統合 (DB 必須)
make -C core cover             # カバレッジ
go test -race ./...            # 競合検出
```

### Kotlin

```bash
cd examples/kotlin-nextjs/backend
./gradlew test
./gradlew jacocoTestReport
./gradlew integrationTest  # 別 task として定義する
```
