# P1: DB 接続基盤 (config / dsn / pgxpool) + docker compose

- 対応 Issue: #21 (M0.3 設計書 #21)
- 親設計書: `docs/specs/21/index.md`
- マイルストーン: M0.3 (Phase 0: スパイク)
- 後続: P2 (マイグレーション基盤) → P3 (健康チェック + 起動統合) → P4 (統合テスト + CI + 規約書)

## 絶対ルール

- **コードベース探索禁止**: Grep, Glob, Read による既存コードの探索的な読み取りを行わない
- **Explore エージェント起動禁止**: サブエージェントによるコードベース調査を行わない
- 実装に必要な情報は全て、以下の context ファイルとこのプロンプト内に記載済み
- context に記載がなく実装判断できない場合は、探索ではなく**ユーザーに質問**すること
- 変更対象ファイルの直接の Read / Edit のみ許可 (探索目的の Read は禁止)
- **Codex レビューをスキップしない**: 各作業ステップに Codex レビューが含まれている。レビューを受けずに次のステップに進むことは禁止
- **完了条件のタスク化**: 作業開始前に「完了条件」セクションの各項目を TaskCreate で登録し、各ステップ完了時に TaskUpdate で状態を更新すること。タスク化せずに作業を開始することは禁止

### プロジェクト固有の禁止事項 (恒久ルール、M0.2 から踏襲)

- **UUID v4 禁止** (`uuid.New` / `uuid.NewRandom` 不可、`uuid.NewV7` のみ許可)
- **`log.Fatal*` 禁止** (`logger.Error` + 非ゼロ終了 = `os.Exit(1)` 等)
- **`time.Local = time.UTC` 禁止** (プロセス全体への副作用、別経路で UTC 化)
- **redact 部分一致禁止** (case-insensitive 完全一致のみ)
- **Domain 層ログ禁止** (`context.Context` から取り出すのみ、ロガー直呼び禁止)
- **`Co-Authored-By:` trailer 不要** (commit message に書かない)
- **main 直 push 禁止** (feature branch + PR + Codex レビュー)

## 作業ステップ (この順序で実行すること)

### ステップ 0: 前提セットアップ

1. M0.2 までマージ済を確認 (`git log --oneline | head -5` で `M0.2` 関連 commit が見える状態)
2. 作業ブランチ `feature/m0.3-impl-p1-db-connection` を `main` から切る
3. `make -C core build && make -C core test && make -C core lint` がベースラインで pass

> **Codex レビュー対象外**: ステップ 0 はブランチ作成・前提確認のみで成果物変更を伴わない。

### ステップ 1: docker compose に PostgreSQL 18.3 service を追加 (F-11 / Q1)

1. `docker/compose.yaml` に PostgreSQL 18.3 service を追加 (本マイルストーンで初の service 追加なら新規ファイル)
2. 設定値:
   - image: `postgres:18.3` (patch まで pin、Q1 確定)
   - container_name / hostname: `id-core-postgres` (or 同等の安定名)
   - environment: `POSTGRES_USER=core` / `POSTGRES_PASSWORD=core_dev_pw` / `POSTGRES_DB=id_core_dev`
   - ports: `5432:5432` (host 側は他と衝突するなら 5532 等に変更可、規約書に明記)
   - volumes: 開発時のデータ永続化 (`postgres_data:/var/lib/postgresql/data`)
   - healthcheck: `pg_isready -U core -d id_core_dev` で 5 秒間隔
3. `docker compose up -d postgres` で起動できることを手元で確認
4. `psql` で接続して `\dt` できることを確認
5. **Codex レビューを実行**
6. 指摘を対応してから次のステップへ

### ステップ 2: `core/internal/config` の DB env 取り込み (F-1 / Q7 / Q10 / Q11)

1. テストを先に書く: `core/internal/config/config_test.go` に DB 関連テスト追加 (T-67〜T-73 該当部分)
   - `CORE_DB_*` 6 個 (接続) + `CORE_DB_POOL_*` 5 個 (プール) のロード成功
   - 不正値 (SSLMODE 不正 / プール負数 / Duration parse エラー) で `Load()` がエラー
   - 既定値 (Q11): MaxConns=10 / MinConns=1 / Lifetime=5m / IdleTime=2m / HealthCheck=30s
2. 実装: `core/internal/config/config.go` の `Config` 構造体に `Database` フィールドを追加
   - 接続: `Host` / `Port` / `User` / `Password` / `DBName` / `SSLMode` (string)
   - プール: `MaxConns` (int32) / `MinConns` (int32) / `MaxConnLifetime` (time.Duration) / `MaxConnIdleTime` (time.Duration) / `HealthCheckPeriod` (time.Duration)
   - SSLMODE バリデーション: 6 種 (`disable`/`allow`/`prefer`/`require`/`verify-ca`/`verify-full`) のみ許容、それ以外は error
   - Duration バリデーション: `time.ParseDuration` で parse、負数禁止
3. `make -C core lint test` 全件 pass
4. **Codex レビューを実行**
5. 指摘を対応してから次のステップへ

### ステップ 3: `core/internal/db` パッケージ新設 (F-1 / F-8 / F-9 / F-10 / F-18)

1. テストを先に書く: `core/internal/db/db_test.go` + `core/internal/db/dsn_test.go` を新規作成
   - DSN 組み立て (T-65 / T-66): 各 SSLMODE 値 / 特殊文字を含む user/password の `url.QueryEscape` 整合
   - DSN redact (T-67): エラーログ時に password が含まれない (logger を mock してログ内容を assert)
   - 接続成功 (T-74) は **Skip 対応** (DB なしでは pass しないので `t.Skip` で skip 可能、CI 統合テストで再実行)
   - ctx cancel (T-80): `Open` の途中で context cancel → 即座に error 返却
2. 実装: 新規ファイル
   - `core/internal/db/db.go`:
     ```go
     // Open creates a *pgxpool.Pool with config from cfg.Database.
     // Returns error if pool creation or initial Ping fails.
     func Open(ctx context.Context, cfg *config.DatabaseConfig, l *logger.Logger) (*pgxpool.Pool, error)
     ```

     - `pgxpool.ParseConfig(dsn)` → プール設定を反映 → `pgxpool.NewWithConfig(ctx, ...)` → `pool.Ping(ctx)`
     - 失敗時のログは host / dbname のみ、password / DSN フルダンプ禁止 (F-10)
     - 失敗時は `*pgxpool.Pool` を `Close` してから error 返却
   - `core/internal/db/dsn.go`:
     ```go
     // BuildDSN は cfg.Database から postgres:// DSN 文字列を組み立てる。
     // user / password は url.QueryEscape でエスケープ、ログ出力時は password を除外する。
     func BuildDSN(cfg *config.DatabaseConfig) string
     // SafeRepr は cfg.Database のうちログ出力可能な項目 (host / dbname / sslmode 等) のみを map で返す。
     func SafeRepr(cfg *config.DatabaseConfig) map[string]any
     ```
3. **`go.mod` への依存追加**: `github.com/jackc/pgx/v5` (採用時点の最新安定版を `go get` で取得後 `go mod tidy`)
4. `make -C core lint test` 全件 pass (DB 接続テストは `t.Skip` で skip される)
5. **Codex レビューを実行**
6. 指摘を対応してから次のステップへ

### ステップ 4: 全体テスト + ベースライン確認 + 最終 Codex レビュー

1. `make -C core build && make -C core test && make -C core lint` 全件 pass
2. `grep -rn 'log\.Fatal' core/` の出力が新規 0 件 (M0.2 で確立した Makefile lint で自動検査)
3. `grep -rn 'uuid\.New(' core/ | grep -v 'NewV7' | grep -v _test.go` の出力が新規 0 件 (UUID v4 不使用)
4. PR 作成 (`gh pr create` with `--assignee` + `--label "種別:実装"` + `--label "対象:基盤"` + `--milestone "M0.3: DB 接続 + マイグレーション基盤"`)
5. **`/pr-codex-review {PR 番号}` で Codex に PR 全体 (差分 + description) をレビューさせる**
6. ゲート通過 (CRITICAL=0 / HIGH=0 / MEDIUM<3) → ユーザー承認 → マージ

## 実装コンテキスト

```
CONTEXT_DIR="docs/context"
```

実装時に参照する context ファイル:

- `${CONTEXT_DIR}/app/architecture.md` (DB 製品 = PostgreSQL 18 を含む全体構成)
- `${CONTEXT_DIR}/backend/conventions.md` (M0.2 までの規約: log.Fatal\* 不使用 / redact deny-list / 環境変数命名 / Makefile lint)
- `${CONTEXT_DIR}/backend/patterns.md` (設定読み込み + main 分離パターン / context への ID 付与パターン)
- `${CONTEXT_DIR}/backend/registry.md` (パッケージマッピング / 環境変数一覧)

設計書: `docs/specs/21/index.md`

適用範囲:

- `core/internal/config/` (既存修正)
- `core/internal/db/` (新規)
- `docker/compose.yaml` (新規 service 追加)
- `core/go.mod` / `core/go.sum` (`pgx/v5` 追加)

## 前提条件

- M0.2 (#7) マージ済 (`core/internal/{logger,apperror,middleware}/` 完成済)
- `make -C core build && test && lint` がベースラインで pass する状態
- 後続 P2 (マイグレーション基盤) は本 P1 の `internal/db.Open` と `BuildDSN` を利用

## 不明点ハンドリング

- 矛盾・欠落・未定義がある場合は作業を止める
- 推測で実装を進めない
- 質問時は: 止まっている作業単位 / 判断が必要な論点 / 選択肢を整理
- 特に以下が判明した時点で**即停止してユーザーに質問**する:
  - PostgreSQL 18.3 image の host 側ポート選定 (5432 vs 5532 等、既存サービスとの衝突回避)
  - 開発用 compose の volume パス (`postgres_data` named volume vs bind mount)
  - `pgx/v5` の取得バージョンが go.mod の go directive と互換性がない場合

## タスク境界

### 本プロンプトで実装する範囲

- `docker/compose.yaml` の PostgreSQL 18.3 service 追加 (F-11 / Q1)
- `core/internal/config/config.go` に DB 関連 env (`CORE_DB_*` 11 個) 取り込み (F-1 / Q7 / Q10 / Q11)
- `core/internal/db/` パッケージ新設 (db.go + dsn.go + 各テスト) (F-1 / F-8 / F-9 / F-10 / F-18)
- `core/go.mod` への `pgx/v5` 追加
- 単体テスト T-65, T-66, T-67, T-68, T-69, T-70, T-71, T-72, T-73 + T-80 (DB 不要部分) を pass

### 本プロンプトでは実装しない範囲

- マイグレーションツール / マイグレーションファイル → P2
- `/health/live` / `/health/ready` エンドポイント → P3
- 起動シーケンス (`cmd/core/main.go`) の更新 → P3 (本 P1 では変更しない)
- 統合テストヘルパー (`internal/testutil/dbtest`) / CI 構成 → P4
- 規約書 (`docs/context/backend/*`) の更新 → P4

## 設計仕様

### 環境変数 (Q7 個別 env 方式 + Q10 SSLMODE + Q11 プール既定値)

| Key                                | 既定値    | 範囲 / 備考                                                                                        |
| ---------------------------------- | --------- | -------------------------------------------------------------------------------------------------- |
| `CORE_DB_HOST`                     | (必須)    | 任意のホスト名 / IP                                                                                |
| `CORE_DB_PORT`                     | (必須)    | `1〜65535` の整数                                                                                  |
| `CORE_DB_USER`                     | (必須)    | PostgreSQL ユーザー                                                                                |
| `CORE_DB_PASSWORD`                 | (必須)    | パスワード (独立 env、F-1 最低ライン)                                                              |
| `CORE_DB_NAME`                     | (必須)    | DB 名                                                                                              |
| `CORE_DB_SSLMODE`                  | `disable` | 6 種: `disable` / `allow` / `prefer` / `require` / `verify-ca` / `verify-full`、それ以外は起動失敗 |
| `CORE_DB_POOL_MAX_CONNS`           | `10`      | 最大接続数、正数のみ                                                                               |
| `CORE_DB_POOL_MIN_CONNS`           | `1`       | 最小接続数 (cold start 回避)                                                                       |
| `CORE_DB_POOL_MAX_CONN_LIFETIME`   | `5m`      | `time.ParseDuration` 互換                                                                          |
| `CORE_DB_POOL_MAX_CONN_IDLE_TIME`  | `2m`      | `time.ParseDuration` 互換                                                                          |
| `CORE_DB_POOL_HEALTH_CHECK_PERIOD` | `30s`     | `time.ParseDuration` 互換                                                                          |

### DSN 組み立て (Q7)

```go
// 形式: postgres://<user>:<password>@<host>:<port>/<dbname>?sslmode=<sslmode>
// user / password は url.QueryEscape でエスケープ
dsn := fmt.Sprintf(
    "postgres://%s:%s@%s:%d/%s?sslmode=%s",
    url.QueryEscape(cfg.User),
    url.QueryEscape(cfg.Password),
    cfg.Host,
    cfg.Port,
    cfg.DBName,
    cfg.SSLMode,
)
```

### redact (F-10)

- 接続失敗ログ: `host` / `port` / `dbname` / `user` / `sslmode` のみ出してよい
- **禁止**: `password` / DSN フルダンプ / 接続文字列のフォーマット済み形式
- 内部実装は `SafeRepr(cfg)` で許可項目だけを map にして logger に渡す

### docker compose (F-11 / Q1)

```yaml
# docker/compose.yaml
services:
  postgres:
    image: postgres:18.3
    container_name: id-core-postgres
    environment:
      POSTGRES_USER: core
      POSTGRES_PASSWORD: core_dev_pw
      POSTGRES_DB: id_core_dev
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U core -d id_core_dev"]
      interval: 5s
      timeout: 3s
      retries: 5

volumes:
  postgres_data:
```

## テスト観点

### 単体テスト (DB 不要)

| #    | 観点                                                                                     | 期待                                                 |
| ---- | ---------------------------------------------------------------------------------------- | ---------------------------------------------------- |
| T-65 | DSN 組み立て (各 SSLMODE 値で正しい DSN 生成)                                            | 想定形式の DSN string                                |
| T-66 | DSN url.QueryEscape (user / password に `@`, `:`, `/`, `?`, `#`, `%` 等)                 | 全特殊文字が `%XX` エンコード、parse 後復元          |
| T-67 | DSN redact (組み立てエラー時のログに password が含まれない)                              | host/dbname のみ、password 文字列なし                |
| T-68 | `CORE_DB_SSLMODE=invalid_value` → `Load()` がエラー                                      | error contains `CORE_DB_SSLMODE`                     |
| T-69 | `CORE_DB_PORT` 不正値                                                                    | バリデーションエラー                                 |
| T-70 | `CORE_DB_POOL_MAX_CONNS=-1` (負数)                                                       | error returned                                       |
| T-71 | `CORE_DB_POOL_MAX_CONN_LIFETIME=invalid` (Duration parse エラー)                         | error returned                                       |
| T-72 | `CORE_DB_POOL_*` 全て空文字 → 既定値反映                                                 | Q11 既定値                                           |
| T-73 | `CORE_DB_POOL_MAX_CONNS=20` → 既定値上書き                                               | `MaxConns=20`                                        |
| T-80 | ctx cancel で Open がキャンセルされる (DB なしテストでは context.WithTimeout で代替検証) | `context.Canceled` 或いは `context.DeadlineExceeded` |

### 統合テスト (DB 必須、本プロンプトでは `t.Skip` で skip 対応、P4 で実機検証)

T-74 接続成功 / T-75〜T-77 接続失敗 / T-78 redact ログ / T-79 プール反映 / T-81 並列 TX / T-82 残留 state — 本 P1 では skeleton 関数のみ用意 (P4 で内容実装)。

## Codex レビューコマンド (各ステップで使用)

```bash
codex exec --full-auto -c model_reasoning_effort="medium" \
  "まず以下の context ファイルを読み取れ:
   - docs/context/app/architecture.md
   - docs/context/backend/conventions.md
   - docs/context/backend/patterns.md
   - docs/specs/21/index.md (Q1 / Q3 / Q7 / Q10 / Q11 / F-1 / F-8 / F-9 / F-10 / F-18)
   その上で git diff をレビューせよ。

   Check:
   1) TDD compliance (テスト先行 → 失敗確認 → 実装 → pass)
   2) DSN 組み立てが Q7 / F-1 と整合 (個別 env / パスワード独立 / url.QueryEscape)
   3) F-10 redact (失敗ログに password / DSN フルダンプが含まれない)
   4) Q10 SSLMODE 6 種バリデーション
   5) Q11 プール既定値 (MaxConns=10 / MinConns=1 / Lifetime=5m / IdleTime=2m / HealthCheck=30s)
   6) F-18 context.Context 必須 (全公開関数の第 1 引数)
   7) UUID v4 / log.Fatal* / Domain 層ログ等の禁止事項違反なし (M0.2 から踏襲)
   8) docker compose の image tag が postgres:18.3 で patch まで pin
   9) Markdown 整合性 / 不要な探索誘発記述なし

   Report issues as CRITICAL/HIGH/MEDIUM/LOW in Japanese with file:line references."
```

PR レベル最終レビュー (`/pr-codex-review {PR 番号}`) も同様の観点で実行する。

## 完了条件

- [ ] M0.2 (#7) マージ済を確認
- [ ] 作業ブランチ `feature/m0.3-impl-p1-db-connection` を作成
- [ ] `docker/compose.yaml` に PostgreSQL 18.3 service を追加 (F-11 / Q1)
- [ ] `docker compose up -d postgres` でローカル起動できることを確認 (psql で `\dt` 可)
- [ ] `core/internal/config/config.go` に `CORE_DB_*` 11 個 (接続 6 + プール 5) の取り込み + バリデーション実装
- [ ] T-65, T-66, T-67, T-68, T-69, T-70, T-71, T-72, T-73 全件 pass
- [ ] `core/internal/db/{db.go, dsn.go}` を新規作成 (Open / BuildDSN / SafeRepr)
- [ ] `go.mod` に `github.com/jackc/pgx/v5` を追加 (`go mod tidy` で go.sum 反映)
- [ ] `make -C core build && make -C core test && make -C core lint` 全件 pass
- [ ] `grep -rn 'log\.Fatal' core/` の出力が新規 0 件 (M0.2 lint で自動検査)
- [ ] 各ステップで Codex レビューを実施、CRITICAL=0 / HIGH=0 / MEDIUM<3
- [ ] PR 作成 (`gh pr create` with `--assignee` + `--label "種別:実装"` + `--label "対象:基盤"` + `--milestone "M0.3: DB 接続 + マイグレーション基盤"`)
- [ ] `/pr-codex-review {番号}` でゲート通過
- [ ] PR の Test plan を実機確認して `[x]` に書き換え
- [ ] PR をマージ
- [ ] 後続 P2 (マイグレーション基盤) に進む
