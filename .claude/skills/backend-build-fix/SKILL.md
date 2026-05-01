---
name: backend-build-fix
description: >-
  バックエンドのビルド/Lint/テストエラーを段階的に修正する。"build-fix", "ビルドエラー", "lint エラー", "コンパイル通らない"
  等で発動。
---
# Backend Build & Fix

バックエンド (`core/` および `examples/*/backend/`) のビルド・Lint・テストエラーを段階的に修正する。

## 適用範囲

- `core/` (Go OIDC OP 本体)
- `examples/go-react/backend/` (Go)
- `examples/kotlin-nextjs/backend/` (Kotlin / Spring Boot)

## 手順

1. **対象ディレクトリを特定** (`core/` か `examples/...` か)
2. **ビルド実行**
   - Go: `make -C {path} build` または `go build ./...`
   - Kotlin: `./gradlew build` (該当ディレクトリで)
3. **エラー出力を解析**:
   - ファイルごとにグループ化
   - 重要度順にソート
4. **各エラーに対して**:
   - エラーコンテキストを表示 (前後 5 行)
   - 問題を説明
   - 修正案を提案
   - 修正を適用
   - ビルドを再実行
5. **ビルド成功後、Lint 実行**:
   - Go: `make lint` または `golangci-lint run ./...`
   - Kotlin: `./gradlew detekt` 等
6. **Lint エラーがあれば同様に修正**
7. **テスト実行**: `make test` 等
8. **必要ならセキュリティスキャン**: `gosec` (Go) など

## よくあるエラーと対処 (Go)

**未使用 import:**

- 不要な import を削除 (`goimports -w` でも可)

**生成コード (sqlc / oapi-codegen) のエラー:**

- 該当の生成元 (`db/queries/*.sql`, `api/openapi.yaml` 等) を更新後、再生成コマンドを実行
- 生成先 (`internal/generated/`) は手動編集禁止

**OIDC ライブラリ関連の型不整合:**

- `go-oidc`, `lestrrat-go/jwx` 等のバージョン整合を確認
- `go.mod` の require / go.sum を再生成

**テスト失敗:**

- テストの分離を確認 (DB 依存テストは TRUNCATE / 専用 DB)
- モックが正しいか検証
- テストではなく**実装側を修正する** (テストを通すために実装を歪めない)

## よくあるエラーと対処 (Kotlin)

**Gradle 依存解決失敗:**

- `./gradlew --refresh-dependencies build` で再取得

**Kotlin compiler の型推論エラー:**

- 明示型注釈を追加して原因を特定

## 原則

- **一度に 1 つのエラーを修正**して再ビルドする (まとめて変更すると原因切り分けが困難)
- ビルドが通らないコードをコミットしない
- Lint 警告を無視せず、必要なら設定で除外する (理由をコメント)
