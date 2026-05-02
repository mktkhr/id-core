# P2: マイグレーション基盤 (golang-migrate + Makefile + dbmigrate + smoke table)

- 対応 Issue: #21 (M0.3 設計書 #21)
- 親設計書: `docs/specs/21/index.md`
- 先行プロンプト: `P1_01_core_db_connection.md`
- マイルストーン: M0.3 (Phase 0: スパイク)
- 後続: P3 (健康チェック + 起動統合) → P4 (統合テスト + CI + 規約書)

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

## 作業ステップ (この順序で実行すること)

### ステップ 0: 前提セットアップ

1. P1 (`feature/m0.3-impl-p1-db-connection` PR) がマージ済を確認
2. 作業ブランチ `feature/m0.3-impl-p2-migrate` を `main` から切る
3. `docker compose up -d postgres` でローカル PostgreSQL 18.3 を起動
4. `make -C core build && make -C core test && make -C core lint` がベースラインで pass

### ステップ 1: golang-migrate CLI のインストール target + Makefile 9 ターゲット追加 (F-3 / Q2)

1. `core/Makefile` に `MIGRATE_VERSION := v4.19.1` 変数を先頭で定義
2. `DB_URL` 変数を **個別 env から組み立て** (Q7 整合):
   ```makefile
   DB_URL ?= postgres://$(CORE_DB_USER):$(CORE_DB_PASSWORD)@$(CORE_DB_HOST):$(CORE_DB_PORT)/$(CORE_DB_NAME)?sslmode=$(CORE_DB_SSLMODE)
   ```
3. 9 ターゲット追加:
   - `migrate-install`: `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION)`
   - `migrate-create`: `NAME` 必須引数チェック → `migrate create -ext sql -dir core/db/migrations -seq -digits 8 $(NAME)` (Q4 = 8 桁連番)
   - `migrate-up`: `migrate -path core/db/migrations -database "$(DB_URL)" up`
   - `migrate-up-one`: `... up 1`
   - `migrate-down`: `... down 1` (1 ステップロールバック)
   - `migrate-down-all`: `... down -all` (全ロールバック、警告表示)
   - `migrate-force`: `VERSION` 必須引数チェック → `... force $(VERSION)` (dirty 復旧)
   - `migrate-version`: `... version`
   - `migrate-status`: `... version 2>&1 || true` (graceful fallback)
4. `make migrate-install` で CLI が `$(go env GOPATH)/bin/migrate` にインストールされることを手元で確認
5. **Codex レビューを実行**
6. 指摘を対応してから次のステップへ

### ステップ 2: スモークテーブルの SQL ファイル作成 (F-2 / Q4 / Q5)

1. ディレクトリ作成: `core/db/migrations/`
2. ファイル作成 (Q4 = 8 桁連番、Q5 = 1 ファイル 1 TX):
   - `core/db/migrations/00000001_smoke_initial.up.sql`:
     ```sql
     CREATE TABLE schema_smoke (
         id    BIGSERIAL PRIMARY KEY,
         label TEXT NOT NULL,
         note  TEXT
     );
     ```
   - `core/db/migrations/00000001_smoke_initial.down.sql`:
     ```sql
     DROP TABLE IF EXISTS schema_smoke;
     ```
3. 環境変数を export して `make migrate-up` を手元実行 → `psql` で `\dt` で `schema_smoke` 存在確認
4. `make migrate-down` で削除確認
5. **Codex レビューを実行**
6. 指摘を対応してから次のステップへ

### ステップ 3: `core/internal/dbmigrate` パッケージ新設 (F-13 / F-14 / F-18 / Q9)

1. テストを先に書く: `core/internal/dbmigrate/migrate_test.go` を新規作成 (T-83〜T-91 該当)
   - T-83 / T-84: migrate up/down 単発成功
   - T-85〜T-87: F-14 double-roundtrip (3 条件 = object 削除 / schema_migrations 状態 / no-error)
   - T-88: 不正 SQL で migrate up が失敗 + dirty 立つ (一時 fixture で再現)
   - T-89 / T-90: AssertClean が dirty → error / clean → nil
   - 統合テスト前提のため、DB 接続失敗時は `t.Skip` (P4 で本格化)
2. 実装: 新規ファイル
   - `core/internal/dbmigrate/migrate.go`:
     ```go
     // RunUp は migrate up を library API 経由で実行する (Makefile の make migrate-up と同等)。
     // 主に F-14 整合テスト用。サーバー起動経路では使わない (Q9: 起動と migrate を分離)。
     func RunUp(ctx context.Context, dsn string, sourceURL string, l *logger.Logger) error
     // RunDown は 1 ステップ分の down を実行する (テスト用)。
     func RunDown(ctx context.Context, dsn string, sourceURL string, l *logger.Logger) error
     // AssertClean は schema_migrations の version + dirty を読み取り、
     // dirty == true なら ErrDirty を返す (F-13 start gate、Q9 起動シーケンスで利用)。
     func AssertClean(ctx context.Context, dsn string, sourceURL string, l *logger.Logger) error
     // ErrDirty は AssertClean が dirty 状態を検出した時のセンチネルエラー。
     var ErrDirty = errors.New("dbmigrate: schema_migrations is dirty (use 'make migrate-force VERSION=<n>' to recover)")
     ```
   - 全公開関数は `ctx context.Context` を第 1 引数に受け取る (F-18)
   - `golang-migrate/migrate/v4` の library API を使う (`migrate.New(sourceURL, dsn)` → `Up()` / `Down()` / `Version()`)
   - エラーログは host / dbname のみ (F-10、SafeRepr 利用)
3. **`go.mod` への依存追加**: `github.com/golang-migrate/migrate/v4`
4. `make -C core lint test` 全件 pass
5. **Codex レビューを実行**
6. 指摘を対応してから次のステップへ

### ステップ 4: 連番衝突検出 lint を Makefile に追加 (Q4 補強)

1. `core/Makefile` の `lint` target に migrations 連番衝突検出を追加:
   ```makefile
   .PHONY: lint
   lint: ## go vet + プロジェクト固有禁止チェック
   	@echo "==> go vet"
   	@go vet ./...
   	@echo "==> log.Fatal* check (project policy)"
   	# (M0.2 で追加した既存検査を維持)
   	@echo "==> migration sequence collision check"
   	@if [ -d core/db/migrations ]; then \
   		dups=$$(ls core/db/migrations 2>/dev/null | awk -F_ '{print $$1}' | sort | uniq -d); \
   		if [ -n "$$dups" ]; then \
   			echo "ERROR: duplicate migration sequence numbers: $$dups"; \
   			exit 1; \
   		fi; \
   	fi
   ```
2. `make -C core lint` が pass することを確認
3. 試験的に `core/db/migrations/00000001_dummy.up.sql` を一時追加して `make -C core lint` が **失敗**することを確認 (確認後に削除)
4. **Codex レビューを実行**
5. 指摘を対応してから次のステップへ

### ステップ 5: 全体テスト + ベースライン確認 + 最終 Codex レビュー

1. `make -C core build && make -C core test && make -C core lint` 全件 pass
2. `make migrate-up` → `make migrate-down` → `make migrate-up` を手動で 1 周して 動作確認
3. PR 作成 (`gh pr create` with `--assignee` + `--label "種別:実装"` + `--label "対象:基盤"` + `--milestone "M0.3"`)
4. **`/pr-codex-review {PR 番号}` で Codex に PR 全体をレビューさせる**
5. ゲート通過 → ユーザー承認 → マージ

## 実装コンテキスト

```
CONTEXT_DIR="docs/context"
```

実装時に参照する context ファイル:

- `${CONTEXT_DIR}/app/architecture.md`
- `${CONTEXT_DIR}/backend/conventions.md` (M0.2 lint / log.Fatal\* 不使用 / redact)
- `${CONTEXT_DIR}/backend/patterns.md` (設定読み込み / context ID 伝播)
- `${CONTEXT_DIR}/backend/registry.md` (パッケージマッピング / 環境変数一覧)

設計書: `docs/specs/21/index.md` (特に Q2 / Q4 / Q5 / Q9 / F-2 / F-3 / F-13 / F-14)

適用範囲:

- `core/db/migrations/` (新規)
- `core/internal/dbmigrate/` (新規)
- `core/Makefile` (9 ターゲット追加 + lint 強化)
- `core/go.mod` (`golang-migrate/migrate/v4` 追加)

## 前提条件

- P1 (`feature/m0.3-impl-p1-db-connection`) マージ済
- `core/internal/db.Open` と `BuildDSN` / `SafeRepr` が利用可能
- `docker compose up -d postgres` でローカル PostgreSQL 18.3 が起動できる
- 後続 P3 (起動シーケンス統合) は本 P2 の `dbmigrate.AssertClean` を `cmd/core/main.go` で利用

## 不明点ハンドリング

- 矛盾・欠落・未定義がある場合は作業を止める
- 推測で実装を進めない
- 質問時は: 止まっている作業単位 / 判断が必要な論点 / 選択肢を整理
- 特に以下が判明した時点で**即停止してユーザーに質問**する:
  - `MIGRATE_VERSION` の最新安定版が `v4.19.1` 以外に判明した場合
  - migrate library API の `migrate.New` のソース URL 形式 (`file://core/db/migrations` の相対パス解決)
  - golang-migrate v4 の Go module path 変更があった場合

## タスク境界

### 本プロンプトで実装する範囲

- `core/Makefile` の 9 ターゲット追加 + 連番衝突 lint 追加 (F-3 / Q2 / Q4)
- `core/db/migrations/00000001_smoke_initial.{up,down}.sql` 新規作成 (F-2 / Q4)
- `core/internal/dbmigrate/{migrate.go, migrate_test.go}` 新規作成 (F-13 / F-14 / F-18 / Q9)
- `go.mod` に `golang-migrate/migrate/v4` 追加
- T-83〜T-91 (DB 必要部分は `t.Skip` で skip 可、CI 統合テストで P4 にて再実行)

### 本プロンプトでは実装しない範囲

- `cmd/core/main.go` の起動シーケンス更新 → P3
- `/health/live` / `/health/ready` エンドポイント → P3
- 統合テストヘルパー (`internal/testutil/dbtest`) → P4
- CI ワークフロー (`.github/workflows/`) 更新 → P4
- 規約書 (`docs/context/backend/*`) の更新 → P4

## 設計仕様

### Makefile 9 ターゲット (F-3 / Q2)

`MIGRATE_VERSION := v4.19.1` を先頭で固定。

| ターゲット                   | コマンド (要点)                                                                                   |
| ---------------------------- | ------------------------------------------------------------------------------------------------- |
| `migrate-install`            | `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION)` |
| `migrate-create NAME=<slug>` | `migrate create -ext sql -dir core/db/migrations -seq -digits 8 $(NAME)`                          |
| `migrate-up`                 | `migrate -path core/db/migrations -database "$(DB_URL)" up`                                       |
| `migrate-up-one`             | `... up 1`                                                                                        |
| `migrate-down`               | `... down 1`                                                                                      |
| `migrate-down-all`           | `... down -all` (警告メッセージ付き)                                                              |
| `migrate-force VERSION=<n>`  | `... force $(VERSION)`                                                                            |
| `migrate-version`            | `... version`                                                                                     |
| `migrate-status`             | `... version 2>&1 \|\| true`                                                                      |

### `internal/dbmigrate` API (F-13 / F-14 / F-18 / Q9)

```go
// RunUp / RunDown はテストおよび補助用途 (主たる migrate 実行は make migrate-up CLI で行う、Q9)
func RunUp(ctx context.Context, dsn string, sourceURL string, l *logger.Logger) error
func RunDown(ctx context.Context, dsn string, sourceURL string, l *logger.Logger) error

// AssertClean はサーバー起動シーケンスから呼ばれる F-13 start gate
func AssertClean(ctx context.Context, dsn string, sourceURL string, l *logger.Logger) error

var ErrDirty = errors.New("dbmigrate: schema_migrations is dirty ...")
```

`sourceURL` は `file://core/db/migrations` 形式 (リポジトリルートからの相対パスで解決)。

### スモークテーブル (F-2 / F-14)

```sql
-- 00000001_smoke_initial.up.sql
CREATE TABLE schema_smoke (
    id    BIGSERIAL PRIMARY KEY,
    label TEXT NOT NULL,
    note  TEXT
);

-- 00000001_smoke_initial.down.sql
DROP TABLE IF EXISTS schema_smoke;
```

### 連番衝突 lint (Q4 補強)

```sh
ls core/db/migrations | awk -F_ '{print $1}' | sort | uniq -d
```

出力 0 件で pass、1 件以上で lint failure。

## テスト観点

### マイグレーションテスト (T-83〜T-91、`internal/dbmigrate/migrate_test.go`)

| #    | 観点                                                                                    | 期待                                                                              |
| ---- | --------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- |
| T-83 | `RunUp(ctx, dsn, sourceURL, l)` でスモークテーブル `schema_smoke` 作成                  | `INFORMATION_SCHEMA.tables` に `schema_smoke` 存在、`schema_migrations.version=1` |
| T-84 | `RunUp` 後に `RunDown` でスモークテーブル削除                                           | `schema_smoke` 削除、`schema_migrations.version` が initial に戻る                |
| T-85 | F-14 (a): double-roundtrip の各段階で object 存在 / 削除 (Up→Down→Up→Down)              | 各 up/down で出現/消失                                                            |
| T-86 | F-14 (b): double-roundtrip 後の `schema_migrations` が initial と一致                   | version=null, dirty=false                                                         |
| T-87 | F-14 (c): double-roundtrip 全工程で no-error                                            | 全関数呼び出しが nil error                                                        |
| T-88 | 不正 SQL を含む一時 migration fixture で `RunUp` がエラー + dirty 立つ                  | error returned, `schema_migrations.dirty=true`                                    |
| T-89 | T-88 の dirty 状態で `AssertClean` が `ErrDirty` 返却                                   | error wraps `ErrDirty`                                                            |
| T-90 | 正常 migrate up 後の `AssertClean` が `nil` 返却                                        | nil                                                                               |
| T-91 | `BEGIN; CREATE TABLE x; COMMIT;` を含む `.up.sql` で `RunUp` がエラー (Q5 (b) 不可確認) | error (二重 TX)                                                                   |

DB 必須テストは `t.Skip()` で skip 可能。本 P2 のローカル CI では skip、P4 で正式に統合テストランナーで実行する。

## Codex レビューコマンド (各ステップで使用)

```bash
codex exec --full-auto -c model_reasoning_effort="medium" \
  "まず以下の context ファイルを読み取れ:
   - docs/context/app/architecture.md
   - docs/context/backend/conventions.md
   - docs/context/backend/patterns.md
   - docs/specs/21/index.md (Q2 / Q4 / Q5 / Q9 / F-2 / F-3 / F-13 / F-14 / F-18)
   その上で git diff をレビューせよ。

   Check:
   1) TDD compliance (テスト先行)
   2) Makefile 9 ターゲットが Q2 完全準拠 + MIGRATE_VERSION pin
   3) Q4 = 8 桁連番命名 (-seq -digits 8) で migrate-create が動く
   4) F-14 double-roundtrip 3 条件をすべて assert
   5) F-13 start gate: AssertClean が dirty 検出で error 返却
   6) F-18 全公開関数が ctx context.Context を第 1 引数で受け取る
   7) Q5 = 1 ファイル 1 TX が暗黙挙動として動作 (BEGIN/COMMIT 明示は不可)
   8) F-10 redact (失敗ログに password / DSN フルダンプ含まない)
   9) lint で連番衝突が検出される
   10) UUID v4 / log.Fatal* / Domain 層ログ等の禁止事項違反なし

   Report issues as CRITICAL/HIGH/MEDIUM/LOW in Japanese with file:line references."
```

## 完了条件

- [ ] P1 (`feature/m0.3-impl-p1-db-connection`) マージ済を確認
- [ ] 作業ブランチ `feature/m0.3-impl-p2-migrate` を作成
- [ ] `core/Makefile` に 9 ターゲット + `MIGRATE_VERSION := v4.19.1` 追加
- [ ] `core/Makefile` の lint で連番衝突検出を追加 (M0.2 既存 lint 検査を維持)
- [ ] `core/db/migrations/00000001_smoke_initial.{up,down}.sql` 新規作成
- [ ] `make migrate-install` でローカルに CLI インストール成功
- [ ] `make migrate-up` → `make migrate-down` → `make migrate-up` の手動 round-trip 成功
- [ ] `core/internal/dbmigrate/{migrate.go, migrate_test.go}` 新規作成 (RunUp / RunDown / AssertClean / ErrDirty)
- [ ] `go.mod` に `golang-migrate/migrate/v4` 追加
- [ ] T-83〜T-91 全件実装、DB 必要分は `t.Skip` で skip
- [ ] `make -C core build && make -C core test && make -C core lint` 全件 pass
- [ ] 各ステップで Codex レビュー、CRITICAL=0 / HIGH=0 / MEDIUM<3
- [ ] PR 作成 + `/pr-codex-review` 通過 + Test plan 実機確認 + マージ
- [ ] 後続 P3 に進む
