# P4: 統合テストヘルパー (testutil/dbtest) + CI ワークフロー + 規約書 4 ファイル更新

- 対応 Issue: #21 (M0.3 設計書 #21)
- 親設計書: `docs/specs/21/index.md`
- 先行プロンプト: `P1_01_core_db_connection.md` / `P2_01_core_dbmigrate.md` / `P3_01_core_health_startup.md`
- マイルストーン: M0.3 (Phase 0: スパイク、本マイルストーンの最終 PR)

## 絶対ルール

- **コードベース探索禁止**: Grep, Glob, Read による既存コードの探索的な読み取りを行わない
- **Explore エージェント起動禁止**: サブエージェントによるコードベース調査を行わない
- 実装に必要な情報は全て、以下の context ファイルとこのプロンプト内に記載済み
- context に記載がなく実装判断できない場合は、探索ではなく**ユーザーに質問**すること
- 変更対象ファイルの直接の Read / Edit のみ許可 (探索目的の Read は禁止)
- **Codex レビューをスキップしない**: 各作業ステップに Codex レビューが含まれている。レビューを受けずに次のステップに進むことは禁止
- **完了条件のタスク化**: 作業開始前に「完了条件」セクションの各項目を TaskCreate で登録し、各ステップ完了時に TaskUpdate で状態を更新すること。タスク化せずに作業を開始することは禁止

### プロジェクト固有の禁止事項 (恒久ルール、M0.2 から踏襲)

- **UUID v4 禁止** / **`log.Fatal*` 禁止** / **`time.Local = time.UTC` 禁止** / **redact 部分一致禁止** / **Domain 層ログ禁止** (詳細は P1 参照)
- **`Co-Authored-By:` trailer 不要**、**main 直 push 禁止**
- **context 文書での「アーカイブ」「特定リポジトリ名」「社内固有事情」記述禁止** (公開リポジトリ前提のテンプレート)

## 作業ステップ (この順序で実行すること)

### ステップ 0: 前提セットアップ

1. P1 / P2 / P3 全てがマージ済を確認 (3 ブランチが main に統合されている)
2. 作業ブランチ `feature/m0.3-impl-p4-testutil-ci-context` を `main` から切る
3. `docker compose up -d postgres` で PostgreSQL 18.3 起動 + `make migrate-up` 反映
4. `make -C core build && make -C core test && make -C core lint` がベースラインで pass

### ステップ 1: `internal/testutil/dbtest` ヘルパー新設 (Q8 / F-17 / F-18)

1. テストを先に書く: `core/internal/testutil/dbtest/helper_test.go` 新規作成 (T-81 / T-82 該当)
   - T-81: 並列 TX 隔離 (subtest で `t.Parallel()` 使用、互いの state を観測しない)
   - T-82: 失敗後の残留 state なし (panic / Rollback で次テストに影響しない)
   - DB 接続失敗時は `t.Skip` で skip
2. 実装: `core/internal/testutil/dbtest/helper.go` 新規作成

   ```go
   // NewPool は TEST_DATABASE_URL から *pgxpool.Pool を生成する。
   //   - TEST_DB_REQUIRED=1 (CI で設定): 接続失敗時に t.Fatal でテストを失敗扱い
   //   - TEST_DB_REQUIRED 未設定 (ローカル): 接続失敗時に t.Skip でスキップ (DB を立てずにユニットテストのみ実行する用途)
   func NewPool(t *testing.T) (context.Context, *pgxpool.Pool)
   // BeginTx は pool.Begin(ctx) で TX を開始 (テスト用)。
   func BeginTx(t *testing.T, ctx context.Context, pool *pgxpool.Pool) pgx.Tx
   // RollbackTx は tx.Rollback(ctx) で TX を巻き戻す (defer 用途)。
   func RollbackTx(t *testing.T, ctx context.Context, tx pgx.Tx)
   ```

   - 全関数は `*testing.T` + `context.Context` を受け取る (F-18 踏襲)
   - `TEST_DATABASE_URL` env が未設定なら fallback 値 (`postgres://core:core_dev_pw@localhost:5432/id_core_test?sslmode=disable`) を使う
   - **CI と Local の挙動分離** (Codex 指摘反映): `TEST_DB_REQUIRED=1` を CI で設定し、CI では DB 接続失敗を取り逃がさない。ローカル開発では未設定のまま `t.Skip` で許容

3. P1 で skeleton 実装した DB 接続テスト (T-74〜T-79) と P2 で skeleton 実装したマイグレーションテスト (T-83〜T-91) のうち DB 必須分を本ヘルパーで本格化
4. **Codex レビューを実行**
5. 指摘を対応してから次のステップへ

### ステップ 2: `make test-integration` ターゲット追加 (Q8 / F-12)

1. `core/Makefile` に `test-integration` ターゲットを追加:

   ```makefile
   TEST_DATABASE_URL ?= postgres://core:core_dev_pw@localhost:5432/id_core_test?sslmode=disable

   .PHONY: test-integration
   test-integration: ## 統合テスト (DB 必要)
   	@echo "🗃️  テスト用 DB を初期化"
   	@migrate -path db/migrations -database "$(TEST_DATABASE_URL)" drop -f
   	@migrate -path db/migrations -database "$(TEST_DATABASE_URL)" up
   	@echo "🧪 統合テスト実行 (build tag = integration)"
   	@TEST_DATABASE_URL="$(TEST_DATABASE_URL)" TEST_DB_REQUIRED=1 go test -p 1 -race -v -tags integration ./...
   ```

   > **パス補足**: `core/Makefile` から見た相対パスのため `migrate -path db/migrations` (`make -C core test-integration` 時の CWD = `core/`)。設計書本文 / プロンプトの context section では repo root 起点の `core/db/migrations` 表記を使う。

2. テスト用 DB (`id_core_test`) は本マイルストーン用にセットアップ手順を規約書に明記
3. ローカルで `make test-integration` が pass することを確認
4. **Codex レビューを実行**
5. 指摘を対応してから次のステップへ

### ステップ 3: CI ワークフロー更新 (F-12 / Q8 / Q9)

1. テストを先に書く: CI 構成変更は実機で観察するため、E2E ベースで検証 (PR 起票時に CI が green になることを確認)
2. 実装: `.github/workflows/` 内の既存 `core/ Go test` ジョブに DB service container を追加
   - `services.postgres`: `image: postgres:18.3`、env (POSTGRES_USER / POSTGRES_PASSWORD / POSTGRES_DB) 設定、health-cmd で readiness 確認
   - step に `make migrate-install` → `make migrate-up` (`DB_URL` を env で渡す) → `make test-integration` を追加
   - `TEST_DATABASE_URL` を service container の host / port から組み立てて env 渡し
3. 既存の `make build` / `make test` (DB 不要) ジョブは並列で動かし、DB-integrated ジョブは順次実行で OK
4. PR 起票時に CI が green になることを確認 (この段階では PR 未起票だが、コミットを push して GitHub Actions の挙動を観察)
5. **Codex レビューを実行**
6. 指摘を対応してから次のステップへ

### ステップ 4-7: `docs/context/` 4 ファイル更新 (1 ステップで実施)

context 4 ファイルは互いに整合性を取る必要があるため、**1 つのコミット単位**として実施し、末尾でまとめて Codex レビューを実行する。

#### 4-1: `docs/context/backend/conventions.md` 更新 (Q12)

1. **「DB / マイグレーション」節 (現状 TBD) を本スコープで詳細化**:
   - DB 製品: PostgreSQL 18.x (Q1)、初期採用 image tag `postgres:18.3`、patch 更新ポリシー
   - クライアントライブラリ: `pgx/v5` 直接 (`pgxpool.Pool`)、sqlc は M2.x で本格導入
   - マイグレーションツール: `golang-migrate/migrate` v4.19.1 (CLI binary 運用)
   - 環境変数 `CORE_DB_*` 11 個一覧 (接続 6 + プール 5、各 env の役割と既定値)
   - マイグレーションファイル命名規則: 8 桁連番 + snake_case slug + `up|down` 拡張子分離
   - トランザクション境界: 1 ファイル = 1 TX、CONCURRENTLY 系は別ファイル分離 + 当面回避
   - SSL/TLS 接続要件: 本番想定で `disable` / `allow` / `prefer` 不可、`require` / `verify-ca` / `verify-full` のみ。本番推奨 `verify-full` (RDS は AWS root CA を `sslrootcert` で指定)
   - 開発者向け運用手順: docker compose / make ターゲット 9 種 / migrate 失敗時復旧 (`make migrate-force VERSION=<n>`)
2. **「ロギング・テレメトリ」節**に DB 経路ログのフィールド (migrate 進捗 / 接続失敗の構造化フィールド) を追記
3. **「環境変数読み込みパターン」節**に `CORE_DB_*` 11 個の取り扱いを追記

#### 4-2: `docs/context/backend/patterns.md` 更新

1. **「DB 接続パターン」節を新設**:
   - `pgxpool.New(ctx, dsn)` → `defer pool.Close()` → `pool.Ping(ctx)` の起動コードサンプル
   - DSN 組み立て (`url.QueryEscape` 利用)
   - redact (`SafeRepr` で許可項目だけログ出力)
   - context.WithTimeout を活用した Ping 制限
2. **「マイグレーション運用パターン」節を新設**:
   - Makefile 9 ターゲットの使い分け
   - 8 桁連番命名規則と `migrate-create` の使い方
   - dirty 状態の手動復旧 (`migrate-force`)
   - F-13 start gate の `dbmigrate.AssertClean` パターン
3. **「統合テストパターン」節を新設**:
   - testify suite + `internal/testutil/dbtest` ヘルパーの使用例
   - `SetupSuite` / `SetupTest` / `TearDownTest` / `TearDownSuite` のライフサイクル
   - `BeginTx` / `RollbackTx` でテスト隔離
   - `-p 1` (package 単位順次実行) の根拠
4. **「context.Context での DB 関連 ID 伝播」節を新設**:
   - DB 接続層・マイグレーション・テストヘルパーの全公開関数が ctx を第 1 引数で受け取る
   - `logger.RequestIDFrom(ctx)` / `logger.EventIDFrom(ctx)` での相関 ID 取得
   - F-14 (Domain 層からロガー直呼び禁止) と D1 順序の踏襲

#### 4-3: `docs/context/backend/registry.md` 更新

1. **パッケージマッピング表に追加**:
   - `core/internal/db` (依存: `pgx/v5`, `pgxpool`, `internal/config`, `internal/logger`)
   - `core/internal/dbmigrate` (依存: `golang-migrate/migrate/v4`, `internal/logger`)
   - `core/internal/testutil/dbtest` (依存: `pgx/v5`, `pgxpool`, `testing`)
2. **環境変数一覧に追加**:
   - `CORE_DB_HOST` / `CORE_DB_PORT` / `CORE_DB_USER` / `CORE_DB_PASSWORD` / `CORE_DB_NAME` / `CORE_DB_SSLMODE` (接続 6 個)
   - `CORE_DB_POOL_MAX_CONNS` / `CORE_DB_POOL_MIN_CONNS` / `CORE_DB_POOL_MAX_CONN_LIFETIME` / `CORE_DB_POOL_MAX_CONN_IDLE_TIME` / `CORE_DB_POOL_HEALTH_CHECK_PERIOD` (プール 5 個)
3. **マイグレーション一覧 (現状 TBD) を本マイルストーン分で埋め始める**:
   - `00000001_smoke_initial`: スモークテーブル `schema_smoke` (本マイルストーン専用、F-14 検証用)

#### 4-4: `docs/context/testing/backend.md` 更新 (M0.2 で M0.2 範囲を埋め済、本スコープで追記)

1. **「DB を要するテスト」節を新設**:
   - tx-rollback ハイブリッドパターン (Q8 / F-17)
   - testify suite + `internal/testutil/dbtest` の利用
   - `make test-integration` ターゲットの説明
   - `-p 1` (package 単位順次実行) の運用根拠
2. **「migrate 整合テストパターン」節を新設**:
   - F-14 double-roundtrip (up→down→up→down) の 3 条件 (object 削除 / schema_migrations 状態 / no-error)
   - dirty フラグ検出 + `AssertClean` 経路のテスト
3. **「`/health/ready` の DB チェックテストパターン」節を追記**:
   - Ping timeout の context.WithTimeout 制御
   - F-7 公開粒度下限のレスポンス検証 (DB 詳細が外部に出ない)
   - F-10 内部ログの redact 検証

#### サブステップ実施後

1. `make -C core lint test` 全件 pass (context 更新は core/ には影響しないが既存テストを壊していないか確認)
2. `npx prettier --check docs/context/backend/*.md docs/context/testing/backend.md` で整形確認 (失敗時は `--write` で整形)
3. **Codex レビューを実行** (4 ファイルの整合性検証を含める)
4. 指摘を対応してから次のステップへ

### ステップ 5: `core/README.md` 更新 (F-16 補強)

1. `core/README.md` に **DB 接続節**を追加:
   - 環境変数 11 個一覧
   - クイックスタート: `docker compose up -d postgres` → `make migrate-install` → `make migrate-up` → `make run` の流れ
   - `/health/live` / `/health/ready` の curl 動作確認例
   - migrate 失敗時の復旧 (`make migrate-force VERSION=<n>`)
2. ディレクトリ構成 (M0.3 時点) を更新 (`internal/db/` / `internal/dbmigrate/` / `internal/testutil/dbtest/` / `db/migrations/` を追加)
3. 主要コマンド表に `make migrate-up` / `make migrate-down` / `make test-integration` 等を追加
4. **Codex レビューを実行**
5. 指摘を対応してから次のステップへ

### ステップ 6: 全体テスト + ベースライン確認 + 最終 Codex レビュー

1. `make -C core build && make -C core test && make -C core lint && make -C core test-integration` 全件 pass
2. `grep -rn 'log\.Fatal' core/` の出力が 0 件 (M0.2 lint で自動検査)
3. CI 全 green
4. PR 作成 (`gh pr create` with `--assignee` + `--label "種別:実装"` + `--label "対象:基盤"` + `--milestone "M0.3"`)
5. **`/pr-codex-review {PR 番号}` で Codex に PR 全体をレビューさせる**
6. ゲート通過 → ユーザー承認 → マージ
7. **M0.3 マイルストーン close** (実装 4 PR 全て完了後にユーザーが実施、または提案)
8. **親要求 Issue #21 を close** (M0.3 完了確認後にユーザーが実施)

## 実装コンテキスト

```
CONTEXT_DIR="docs/context"
```

実装時に参照する context ファイル (本フェーズで更新する側):

- `${CONTEXT_DIR}/app/architecture.md`
- `${CONTEXT_DIR}/backend/conventions.md` (本フェーズで更新)
- `${CONTEXT_DIR}/backend/patterns.md` (本フェーズで更新)
- `${CONTEXT_DIR}/backend/registry.md` (本フェーズで更新)
- `${CONTEXT_DIR}/testing/backend.md` (本フェーズで更新)

設計書: `docs/specs/21/index.md` (特に「既存資料からの差分」節)

適用範囲:

- `core/internal/testutil/dbtest/{helper.go, helper_test.go}` (新規)
- `core/Makefile` (`test-integration` ターゲット追加)
- `.github/workflows/` (DB service container + migrate up + test-integration step 追加)
- `docs/context/backend/{conventions,patterns,registry}.md` + `docs/context/testing/backend.md` (既存修正)
- `core/README.md` (DB 接続節追加)

## 前提条件

- P1 / P2 / P3 全てマージ済
- `core/internal/{db,dbmigrate}/` および `core/internal/health/{live,ready}.go`、`core/cmd/core/main.go` (起動シーケンス) が完成済
- 後続マイルストーン (M0.4 以降 / M1.x / M2.x) は本 P4 で確立した規約書 + 統合テストヘルパーを再利用

## 不明点ハンドリング

- 矛盾・欠落・未定義がある場合は作業を止める
- 推測で実装を進めない
- 質問時は: 止まっている作業単位 / 判断が必要な論点 / 選択肢を整理
- 特に以下が判明した時点で**即停止してユーザーに質問**する:
  - GitHub Actions の `services.postgres` の最新構文 (env / health-cmd / port mapping の仕様変更)
  - `migrate -path` の CI 上の relative path 解決 (workdir との関係)
  - prettier の DB 識別子 (`db_name` / `CORE_DB_*`) の `_*` パターン誤整形 (M0.2 で発生事例あり、`dbname` のように `_` を避ける表記でも可)

## タスク境界

### 本プロンプトで実装する範囲

- `core/internal/testutil/dbtest/{helper.go, helper_test.go}` 新規 (Q8 / F-17 / F-18)
- `core/Makefile` の `test-integration` ターゲット追加 (Q8 / F-12)
- `.github/workflows/` 更新 (DB service container + migrate + test-integration、F-12 / Q8 / Q9)
- `docs/context/backend/{conventions,patterns,registry}.md` 更新 (F-16 規約書 / Q12)
- `docs/context/testing/backend.md` 更新 (F-14 / F-17 / Q8)
- `core/README.md` 更新 (DB 接続節、F-16 入口段落)
- T-74〜T-82 (P1 / P2 で skip 対応した DB 必須テスト) を本格化
- T-101 までの全テストが pass する状態を最終確認

### 本プロンプトでは実装しない範囲

- 実テーブル DDL (アカウント / クライアント / トークン等) → M2.x / M3.x
- sqlc 連携 → M2.x
- 本番想定 migrate ターゲット (`migrate-remote-*`) → 別マイルストーン
- 監視・メトリクス連携 → M1.x 以降

## 設計仕様

### `internal/testutil/dbtest/helper.go` (Q8 / F-17 / F-18)

```go
package dbtest

const defaultTestDatabaseURL = "postgres://core:core_dev_pw@localhost:5432/id_core_test?sslmode=disable"

func DatabaseURL() string {
    if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
        return dsn
    }
    return defaultTestDatabaseURL
}

// dbRequired は CI で TEST_DB_REQUIRED=1 が設定されているかを判定する。
func dbRequired() bool {
    return os.Getenv("TEST_DB_REQUIRED") == "1"
}

func NewPool(t *testing.T) (context.Context, *pgxpool.Pool) {
    t.Helper()
    ctx := context.Background()
    pool, err := pgxpool.New(ctx, DatabaseURL())
    if err != nil {
        if dbRequired() {
            t.Fatalf("テスト DB 接続に失敗しました (TEST_DB_REQUIRED=1): %v", err)
        }
        t.Skipf("テスト DB に接続できないためスキップします: %v", err)
    }
    if err := pool.Ping(ctx); err != nil {
        if dbRequired() {
            t.Fatalf("テスト DB Ping に失敗しました (TEST_DB_REQUIRED=1): %v", err)
        }
        t.Skipf("テスト DB Ping 失敗のためスキップします: %v", err)
    }
    return ctx, pool
}

func BeginTx(t *testing.T, ctx context.Context, pool *pgxpool.Pool) pgx.Tx {
    t.Helper()
    tx, err := pool.Begin(ctx)
    if err != nil {
        t.Fatalf("BeginTx: %v", err)
    }
    return tx
}

func RollbackTx(t *testing.T, ctx context.Context, tx pgx.Tx) {
    t.Helper()
    if tx == nil {
        return
    }
    if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
        t.Errorf("RollbackTx: %v", err)
    }
}
```

### `make test-integration` ターゲット (Q8 / F-12)

```makefile
TEST_DATABASE_URL ?= postgres://core:core_dev_pw@localhost:5432/id_core_test?sslmode=disable

.PHONY: test-integration
test-integration: ## 統合テスト (DB 必要、build tag=integration)
	@echo "🗃️  テスト用 DB を初期化"
	@migrate -path db/migrations -database "$(TEST_DATABASE_URL)" drop -f
	@migrate -path db/migrations -database "$(TEST_DATABASE_URL)" up
	@echo "🧪 統合テスト実行 (build tag = integration)"
	@TEST_DATABASE_URL="$(TEST_DATABASE_URL)" TEST_DB_REQUIRED=1 go test -p 1 -race -v -tags integration ./...
```

統合テストファイルは先頭に `//go:build integration` ビルドタグを付与する (例: `core/internal/db/db_integration_test.go` の先頭)。これにより `go test ./...` (タグ無し) では除外、`go test -tags integration ./...` でのみ実行される構成。命名規則 (`*IntegrationTest*`) 依存より頑健。

### CI ワークフロー (F-12)

`.github/workflows/` 内の既存 `core/ Go test` ジョブに以下を追加:

```yaml
services:
  postgres:
    image: postgres:18.3
    env:
      POSTGRES_USER: core
      POSTGRES_PASSWORD: core_dev_pw
      POSTGRES_DB: id_core_test
    ports:
      - 5432:5432
    options: >-
      --health-cmd="pg_isready -U core -d id_core_test"
      --health-interval=5s
      --health-timeout=3s
      --health-retries=10
steps:
  # 既存の go test step の前に
  - name: Install golang-migrate
    run: make -C core migrate-install
  - name: Run migrate up
    run: make -C core migrate-up
    env:
      CORE_DB_HOST: localhost
      CORE_DB_PORT: 5432
      CORE_DB_USER: core
      CORE_DB_PASSWORD: core_dev_pw
      CORE_DB_NAME: id_core_test
      CORE_DB_SSLMODE: disable
  - name: Run integration tests
    run: make -C core test-integration
    env:
      TEST_DATABASE_URL: postgres://core:core_dev_pw@localhost:5432/id_core_test?sslmode=disable
      TEST_DB_REQUIRED: "1" # CI では DB 必須、Skip は許容しない
```

## テスト観点

### 統合テスト本格化 (T-74〜T-82、`make test-integration` 経由で実行)

| #    | 観点                                                                                   | 期待                                                 |
| ---- | -------------------------------------------------------------------------------------- | ---------------------------------------------------- |
| T-74 | 接続成功 (compose の PostgreSQL 18.3 に Open + Ping 成功)                              | `*pgxpool.Pool` 取得、Ping nil error                 |
| T-75 | 接続失敗 (host 不正)                                                                   | error returned within timeout                        |
| T-76 | 接続失敗 (password 不正)                                                               | error returned, ログに password 含まれない           |
| T-77 | 接続失敗 (DB 停止)                                                                     | error returned                                       |
| T-78 | T-75〜T-77 のエラーログ                                                                | host/dbname のみ、password / DSN フルダンプなし      |
| T-79 | プール設定反映 (CORE_DB_POOL_MAX_CONNS=5 で起動 → pool.Stat() で確認)                  | 5                                                    |
| T-80 | ctx cancel で Open がキャンセル (DeadlineExceeded)                                     | `context.Canceled` または `context.DeadlineExceeded` |
| T-81 | 並列 TX 隔離 (同一 pool から 2 subtest が `t.Parallel()` で別 INSERT → 互いに見えない) | 互いの state を観測しない                            |
| T-82 | テスト失敗時の残留 state なし (panic → defer Rollback → 次テストで clean state)        | 2 つ目のテストで残留 INSERT 不可視                   |

### マイグレーションテスト本格化 (T-83〜T-91、本 P4 で skip → 実機実行に切り替え)

P2 で skeleton 化した T-83〜T-91 を `internal/testutil/dbtest` の `NewPool` 経由で実機実行する。

### HTTP エンドポイントテスト本格化 (T-94〜T-98)

P3 で実装した `/health/ready` の DB チェックテストを `internal/testutil/dbtest` 経由で実機 DB に対して実行 (P3 では mocked pool で skeleton)。

## Codex レビューコマンド (各ステップで使用)

```bash
codex exec --full-auto -c model_reasoning_effort="medium" \
  "まず以下の context ファイルを読み取れ:
   - docs/context/app/architecture.md
   - docs/context/backend/conventions.md (本フェーズで更新)
   - docs/context/backend/patterns.md (本フェーズで更新)
   - docs/context/backend/registry.md (本フェーズで更新)
   - docs/context/testing/backend.md (本フェーズで更新)
   - docs/specs/21/index.md (Q1 / Q2 / Q3 / Q4 / Q5 / Q6 / Q7 / Q8 / Q9 / Q10 / Q11 / Q12 全件)
   その上で git diff をレビューせよ。

   Check:
   1) Q8 = tx-rollback ハイブリッドが BeginTx / RollbackTx ヘルパーで実装されている
   2) F-17 並列隔離 (T-81 / T-82) の実機検証が動作する
   3) F-12 CI ワークフローで postgres 18.3 service container + migrate up + test-integration が順序通り
   4) F-16 規約書最低必須項目 5 件が conventions.md の DB / マイグレーション節に集約されている
   5) Q12 = conventions.md / patterns.md / registry.md / testing.md の 4 ファイル整合
   6) F-18 ctx 必須が新規ヘルパー全公開関数で守られている
   7) F-10 redact が DB 失敗ログ全経路で守られている (host/dbname のみ、password / DSN なし)
   8) UUID v4 / log.Fatal* / Domain 層ログ等の禁止事項違反なし
   9) アーカイブ参照や特定リポジトリ名の混入がない (公開リポジトリ前提)
   10) prettier 整形で db_name / CORE_DB_* 等の `_*` パターンが破壊されていない (M0.2 で発生した事例の再発防止)

   Report issues as CRITICAL/HIGH/MEDIUM/LOW in Japanese with file:line references."
```

## 完了条件

- [ ] P1 / P2 / P3 全てマージ済を確認
- [ ] 作業ブランチ `feature/m0.3-impl-p4-testutil-ci-context` を作成
- [ ] `core/internal/testutil/dbtest/{helper.go, helper_test.go}` 新規作成 (NewPool / BeginTx / RollbackTx)
- [ ] T-81 / T-82 (並列 TX 隔離 + 残留 state なし) pass
- [ ] `core/Makefile` に `test-integration` ターゲット追加 (drop+migrate up → `go test -p 1 ...`)
- [ ] `.github/workflows/` に postgres 18.3 service + migrate up + test-integration step 追加
- [ ] CI が PR 起票時に green (DB 統合テスト含む)
- [ ] `docs/context/backend/conventions.md` の DB / マイグレーション節を詳細化 (F-16 5 必須項目)
- [ ] `docs/context/backend/patterns.md` に 4 節新設 (DB 接続 / マイグレーション運用 / 統合テスト / context ID 伝播)
- [ ] `docs/context/backend/registry.md` にパッケージ 3 件 + 環境変数 11 個 + マイグレーション 1 件 (smoke_initial) を追加
- [ ] `docs/context/testing/backend.md` に DB を要するテスト節 + migrate 整合テスト節 + `/health/ready` テスト節を追加
- [ ] `core/README.md` に DB 接続節 + クイックスタート + ディレクトリ構成更新
- [ ] T-74〜T-101 (M0.3 全 37 ケース) が `make test` (DB 不要) または `make test-integration` (DB 必要) で全件 pass
- [ ] `make -C core build && make -C core test && make -C core lint && make -C core test-integration` 全件 pass
- [ ] 各ステップで Codex レビュー、CRITICAL=0 / HIGH=0 / MEDIUM<3
- [ ] PR 作成 + `/pr-codex-review` 通過 + Test plan 実機確認 + マージ
- [ ] **M0.3 マイルストーンを close** (P4 マージで実装 4 PR 完了 → ユーザー承認後)
- [ ] **親要求 Issue #21 を close** (M0.3 完了確認後にユーザーが実施)
