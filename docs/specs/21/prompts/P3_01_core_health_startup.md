# P3: /health/live + /health/ready エンドポイント + 起動シーケンス統合

- 対応 Issue: #21 (M0.3 設計書 #21)
- 親設計書: `docs/specs/21/index.md`
- 先行プロンプト: `P1_01_core_db_connection.md` / `P2_01_core_dbmigrate.md`
- マイルストーン: M0.3 (Phase 0: スパイク)
- 後続: P4 (統合テスト + CI + 規約書)

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

1. P1 (`feature/m0.3-impl-p1-db-connection`) と P2 (`feature/m0.3-impl-p2-migrate`) が両方マージ済を確認
2. 作業ブランチ `feature/m0.3-impl-p3-health-startup` を `main` から切る
3. `docker compose up -d postgres` で PostgreSQL 18.3 起動 + `make migrate-up` でスキーマ反映
4. `make -C core build && make -C core test && make -C core lint` がベースラインで pass

### ステップ 1: `/health/live` エンドポイント追加 (Q6)

1. テストを先に書く: `core/internal/health/live_test.go` 新規作成 (T-93 該当)
   - `GET /health/live` → 200 + `{"status":"ok"}`
   - DB 状態に依存しない (DB 停止状態でも 200)
2. 実装: `core/internal/health/live.go` 新規作成
   - `func NewLiveHandler(l *logger.Logger) http.Handler`
   - DB チェックなし、固定 200 + `{"status":"ok"}` を返却
3. `core/internal/server/server.go` に `mux.HandleFunc("GET /health/live", health.NewLiveHandler(l).ServeHTTP)` を追加
4. `make -C core lint test` 全件 pass
5. **Codex レビューを実行**
6. 指摘を対応してから次のステップへ

### ステップ 2: `/health/ready` エンドポイント追加 (Q6 / F-7 / F-10)

1. テストを先に書く: `core/internal/health/ready_test.go` 新規作成 (T-94〜T-98 該当)
   - T-94: pool.Ping 成功 → 200 + `{"status":"ok"}`
   - T-95: pool.Ping 失敗 → 503 + `{"status":"unavailable"}`
   - T-96: 503 レスポンスに DB 製品名 / バージョン / ホスト / DSN / エラー詳細が含まれない
   - T-97: アプリ側 timeout 2 秒で 503 (DB が遅い場合)
   - T-98: 503 時の内部 ERROR ログに host/dbname のみ、password / DSN フルダンプなし
2. 実装: `core/internal/health/ready.go` 新規作成
   - `func NewReadyHandler(pool *pgxpool.Pool, l *logger.Logger) http.Handler`
   - リクエスト受信 → `ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)` → `pool.Ping(ctx)`
   - 成功時: 200 + `{"status":"ok"}`
   - 失敗時: `l.Error(ctx, "DB readiness check failed", err, ...)` (host/dbname のみ含むログ) → 503 + `{"status":"unavailable"}`
   - レスポンス JSON はそれぞれ固定 (DB 製品名等を含めない、F-7 公開粒度下限)
3. `core/internal/server/server.go` の `New(cfg, l)` シグネチャを `New(cfg, l, pool)` に変更し、`mux.HandleFunc("GET /health/ready", health.NewReadyHandler(pool, l).ServeHTTP)` を追加
4. `make -C core lint test` 全件 pass
5. **Codex レビューを実行**
6. 指摘を対応してから次のステップへ

### ステップ 3: 起動シーケンスの更新 (`cmd/core/main.go`、F-6 / F-13 / Q9)

1. テストを先に書く: `core/cmd/core/main_test.go` を更新 (T-99〜T-101 該当)
   - T-99: 正常起動シーケンス (logger → pgxpool → Ping → AssertClean → server) の関数構造テスト
   - T-100: DB 接続失敗 → logger.Error + 非ゼロ終了 (起動シーケンスの中で error path を assert)
   - T-101: dirty 検出 → 起動拒否 (mock で AssertClean を ErrDirty にして起動シーケンスをテスト)
2. 実装: `core/cmd/core/main.go` の起動シーケンスを以下に更新
   ```
   1. logger.Default() (CORE_LOG_FORMAT 反映)
   2. config.Load() (CORE_DB_* 含む)
   3. ctx := logger.WithEventID(context.Background(), uuid.NewV7().String())
   4. dbPool, err := db.Open(ctx, &cfg.Database, l)
      err != nil → l.Error(...) + os.Exit(1) (F-6)
      defer dbPool.Close()
   5. err := dbmigrate.AssertClean(ctx, db.BuildDSN(&cfg.Database), "file://core/db/migrations", l)
      err != nil → l.Error(...) + os.Exit(1) (F-13 start gate)
   6. srv := server.New(cfg, l, dbPool)
   7. signal handling + ListenAndServe (M0.2 既存パターン踏襲)
   ```
3. M0.2 で確立した M0.2 の `bootstrap()` / `run()` 分離パターンを維持
4. `grep -rn 'log\.Fatal' core/` の出力が 0 件 (M0.2 lint で自動検査)
5. `make -C core build && make -C core test && make -C core lint` 全件 pass
6. 手動動作確認:
   - 正常起動 (DB 起動済 + clean state) → サーバ起動成功 + `curl /health/live` / `/health/ready` で 200
   - DB 停止状態で起動 → 非ゼロ exit、構造化エラーログ出力
   - `make migrate-force VERSION=999` で意図的に dirty 状態 → 起動拒否、構造化エラーログ → `make migrate-force VERSION=1` で復旧
7. **Codex レビューを実行**
8. 指摘を対応してから次のステップへ

### ステップ 4: 全体テスト + ベースライン確認 + 最終 Codex レビュー

1. `make -C core build && make -C core test && make -C core lint` 全件 pass
2. PR 作成 (`gh pr create` with `--assignee` + `--label "種別:実装"` + `--label "対象:基盤"` + `--milestone "M0.3"`)
3. **`/pr-codex-review {PR 番号}` で Codex に PR 全体をレビューさせる**
4. ゲート通過 → ユーザー承認 → マージ

## 実装コンテキスト

```
CONTEXT_DIR="docs/context"
```

実装時に参照する context ファイル:

- `${CONTEXT_DIR}/app/architecture.md`
- `${CONTEXT_DIR}/backend/conventions.md` (M0.2 ログ規約 / log.Fatal\* 不使用 / redact)
- `${CONTEXT_DIR}/backend/patterns.md` (HTTP middleware D1 順序 / 設定読み込み + main 分離パターン)
- `${CONTEXT_DIR}/backend/registry.md`

設計書: `docs/specs/21/index.md` (特に Q6 / F-6 / F-7 / F-10 / F-13 / F-18 / Q9)

適用範囲:

- `core/internal/health/{live.go, live_test.go, ready.go, ready_test.go}` (新規)
- `core/internal/server/server.go` (シグネチャ変更 + ハンドラ登録追加)
- `core/cmd/core/main.go` (起動シーケンス更新)
- `core/cmd/core/main_test.go` (起動シーケンステスト更新)

## 前提条件

- P1 (`feature/m0.3-impl-p1-db-connection`) マージ済 → `core/internal/db.{Open,BuildDSN,SafeRepr}` 利用可能
- P2 (`feature/m0.3-impl-p2-migrate`) マージ済 → `core/internal/dbmigrate.{AssertClean,ErrDirty}` 利用可能
- 後続 P4 は本 P3 の handler / 起動シーケンスを統合テストの対象として利用

## 不明点ハンドリング

- 矛盾・欠落・未定義がある場合は作業を止める
- 推測で実装を進めない
- 質問時は: 止まっている作業単位 / 判断が必要な論点 / 選択肢を整理
- 特に以下が判明した時点で**即停止してユーザーに質問**する:
  - `server.New` の既存シグネチャ (M0.2 で `New(cfg, l)`) を `New(cfg, l, pool)` に変更する破壊的変更が他箇所に影響する場合
  - `bootstrap` / `run` 分離パターンの構造を起動シーケンスに自然に組み込めない場合
  - signal handling との順序関係 (DB 接続前 vs 後) で複雑な分岐が必要な場合

## タスク境界

### 本プロンプトで実装する範囲

- `core/internal/health/live.{go,_test.go}` 新規 (T-93、Q6 `/health/live`)
- `core/internal/health/ready.{go,_test.go}` 新規 (T-94〜T-98、Q6 `/health/ready` + F-7 公開粒度 + F-10 redact)
- `core/internal/server/server.go` のシグネチャ変更 + ハンドラ登録 (`New(cfg, l, pool)`)
- `core/cmd/core/main.go` 起動シーケンス更新 (F-6 / F-13 / Q9 / F-18)
- `core/cmd/core/main_test.go` 起動シーケンステスト更新 (T-99〜T-101)
- 既存 `/health` (M0.1 から) は外形互換のまま維持 (T-92)

### 本プロンプトでは実装しない範囲

- 統合テストヘルパー (`internal/testutil/dbtest`) → P4
- CI ワークフロー (`.github/workflows/`) 更新 → P4
- 規約書 (`docs/context/backend/*`) の更新 → P4
- T-74〜T-82 の完全な統合テスト本格実装 → P4 (本 P3 では `t.Skip` で skip 可)

## 設計仕様

### `/health/live` (Q6)

- Method: GET
- Auth: 不要 (公開)
- 200 レスポンス: `{"status":"ok"}` 固定 (DB チェックなし)
- 503: 返さない (プロセス死亡時は TCP 切断のみ)
- 用途: k8s livenessProbe / プロセス疎通確認

### `/health/ready` (Q6 / F-7 / F-10)

- Method: GET
- Auth: 不要 (公開)
- 200 レスポンス: `{"status":"ok"}` (DB 接続成功時のみ)
- 503 レスポンス: `{"status":"unavailable"}` (DB 接続失敗時)
- DB チェック: `pgxpool.Pool.Ping(ctx)` with `2 秒` の context.WithTimeout
- 公開粒度下限 (F-7): どの依存先がダメか / DB 製品名 / バージョン / ホスト / 内部エラー詳細は外部に出さない
- 内部 ERROR ログ (F-10): host / dbname のみ含み、password / DSN フルダンプ禁止

### 起動シーケンス (F-6 / F-13 / Q9 / F-18)

```
make run / ./bin/core
 → logger.Default() (CORE_LOG_FORMAT 反映)
 → config.Load() (env CORE_DB_* 含む)
 → ctx := logger.WithEventID(context.Background(), uuid.NewV7().String())  // F-18
 → dbPool, err := db.Open(ctx, &cfg.Database, l)
    err != nil → l.Error(ctx, "DB 接続失敗", err, db.SafeRepr(&cfg.Database)...) + os.Exit(1)
    defer dbPool.Close()
 → err := dbmigrate.AssertClean(ctx, db.BuildDSN(&cfg.Database), "file://core/db/migrations", l)
    err != nil →
       errors.Is(err, dbmigrate.ErrDirty) →
          l.Error(ctx, "schema_migrations is dirty: run 'make migrate-force VERSION=<n>' to recover", err) + os.Exit(1)
       else →
          l.Error(ctx, "AssertClean failed", err) + os.Exit(1)
 → srv := server.New(cfg, l, dbPool)
 → signal handling (SIGTERM, SIGINT for graceful shutdown, M0.2 既存)
 → srv.ListenAndServe()
```

### `server.New` シグネチャ変更

```go
// M0.2: func New(cfg *config.Config, l *logger.Logger) *http.Server
// M0.3:
func New(cfg *config.Config, l *logger.Logger, pool *pgxpool.Pool) *http.Server {
    mux := http.NewServeMux()
    mux.HandleFunc("GET /health", health.NewHandler(l).ServeHTTP)         // 既存維持
    mux.HandleFunc("GET /health/live", health.NewLiveHandler(l).ServeHTTP) // 新規 (Q6)
    mux.HandleFunc("GET /health/ready", health.NewReadyHandler(pool, l).ServeHTTP) // 新規 (Q6)

    handler := middleware.Recover(l, mux)
    handler = middleware.AccessLog(l, handler)
    handler = middleware.RequestID(handler)

    return &http.Server{Addr: cfg.Addr(), Handler: handler}
}
```

## テスト観点

### HTTP エンドポイントテスト (T-92〜T-98)

| #    | 観点                                                                               | 期待                                               |
| ---- | ---------------------------------------------------------------------------------- | -------------------------------------------------- |
| T-92 | `GET /health` (既存 M0.1) → 後方互換                                               | 200 + `{"status":"ok"}` (DB 状態反映なし)          |
| T-93 | `GET /health/live` (DB 状態に関わらず)                                             | 200 + `{"status":"ok"}`                            |
| T-94 | `GET /health/ready` (pool.Ping 成功)                                               | 200 + `{"status":"ok"}`                            |
| T-95 | `GET /health/ready` (DB 停止 / pool.Ping 失敗)                                     | 503 + `{"status":"unavailable"}`                   |
| T-96 | T-95 のレスポンスに DB 製品名 / バージョン / ホスト / DSN / エラー詳細が含まれない | レスポンス body は `{"status":"unavailable"}` のみ |
| T-97 | DB が応答に 3 秒以上かかる状態 → アプリ側 2 秒 timeout で 503                      | 503 + `{"status":"unavailable"}`                   |
| T-98 | T-95 / T-97 の内部 ERROR ログを capture                                            | host/dbname のみ、password / DSN なし              |

### 起動シーケンステスト (T-99〜T-101)

| #     | 観点                                                                | 期待                                                                 |
| ----- | ------------------------------------------------------------------- | -------------------------------------------------------------------- |
| T-99  | 正常起動シーケンス (logger → pgxpool → Ping → AssertClean → server) | 関数構造の順序が期待通り、ListenAndServe まで到達                    |
| T-100 | DB 停止状態で起動 → 非ゼロ exit                                     | `os.Exit(1)` 相当、`logger.Error("DB 接続失敗", err, ...)` 出力      |
| T-101 | dirty 状態で起動 → 起動拒否                                         | `os.Exit(1)` 相当、`logger.Error("schema_migrations is dirty", ...)` |

## Codex レビューコマンド (各ステップで使用)

```bash
codex exec --full-auto -c model_reasoning_effort="medium" \
  "まず以下の context ファイルを読み取れ:
   - docs/context/app/architecture.md
   - docs/context/backend/conventions.md
   - docs/context/backend/patterns.md
   - docs/specs/21/index.md (Q6 / F-6 / F-7 / F-10 / F-13 / F-18 / Q9)
   その上で git diff をレビューせよ。

   Check:
   1) TDD compliance (テスト先行)
   2) Q6 = /health/live + /health/ready 分割、既存 /health は外形互換維持
   3) F-7 公開粒度下限: 503 ボディは固定文字列のみ、DB 詳細を露出しない
   4) F-10 redact: 内部 ERROR ログに password / DSN フルダンプを含まない
   5) F-6 / F-13 起動拒否: DB 接続失敗 / dirty 検出時の logger.Error + 非ゼロ終了
   6) F-18 ctx 必須: 全公開関数の第 1 引数が context.Context
   7) Q9 起動シーケンス: 順序 (logger → pgxpool → Ping → AssertClean → server)
   8) UUID v7 のみ使用 (uuid.NewV7、v4 禁止)
   9) log.Fatal* 不使用 (M0.2 lint で自動検査されるが、念のため)
   10) M0.2 確立の bootstrap / run 分離パターンを維持
   11) ready timeout 2 秒の context.WithTimeout 実装

   Report issues as CRITICAL/HIGH/MEDIUM/LOW in Japanese with file:line references."
```

## 完了条件

- [ ] P1 / P2 マージ済を確認
- [ ] 作業ブランチ `feature/m0.3-impl-p3-health-startup` を作成
- [ ] `core/internal/health/live.{go,_test.go}` 新規作成、T-93 pass
- [ ] `core/internal/health/ready.{go,_test.go}` 新規作成、T-94〜T-98 pass
- [ ] `core/internal/server/server.go` のシグネチャを `New(cfg, l, pool)` に変更、3 ハンドラ登録 (`/health` / `/health/live` / `/health/ready`)
- [ ] `core/cmd/core/main.go` 起動シーケンス更新 (logger → pgxpool → Ping → AssertClean → server)
- [ ] `core/cmd/core/main_test.go` 更新、T-99〜T-101 pass
- [ ] T-92 (既存 `/health` 後方互換) も pass
- [ ] `make -C core build && make -C core test && make -C core lint` 全件 pass
- [ ] `grep -rn 'log\.Fatal' core/` 出力 0 件 (M0.2 lint 自動検査で保証)
- [ ] `grep -rn 'uuid\.New(' core/ | grep -v 'NewV7' | grep -v _test.go` 出力 0 件
- [ ] 手動動作確認: 正常起動 / DB 停止 / dirty 状態の 3 シナリオで挙動を確認
- [ ] 各ステップで Codex レビュー、CRITICAL=0 / HIGH=0 / MEDIUM<3
- [ ] PR 作成 + `/pr-codex-review` 通過 + Test plan 実機確認 + マージ
- [ ] 後続 P4 に進む
