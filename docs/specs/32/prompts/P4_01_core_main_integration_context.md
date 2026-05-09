# P4_01: core/cmd/core/main + server route 統合 + 統合テスト + context 更新

M1.1 の **配線** タスク。P1 (config + keystore + devkeygen) / P2 (Discovery) / P3 (JWKS + notimpl) で実装した部品を `main.go` + `server.go` に統合し、起動シーケンスへ keystore 初期化 + WARN ログを追加、統合テスト (env 切替) を実装、`docs/context/` を最終化する。

## 絶対ルール

- **コードベース探索禁止**: Grep, Glob, Read による既存コードの探索的な読み取りを行わない
- **Explore エージェント起動禁止**: サブエージェントによるコードベース調査を行わない
- 実装に必要な情報は全て、以下の context ファイルとこのプロンプト内に記載済み
- context に記載がなく実装判断できない場合は、探索ではなく**ユーザーに質問**すること
- 変更対象ファイルの直接の Read / Edit のみ許可 (探索目的の Read は禁止)
- **Codex レビューをスキップしない**: 各作業ステップに Codex レビューが含まれている。レビューを受けずに次のステップに進むことは禁止
- **完了条件のタスク化**: 作業開始前に「完了条件」セクションの各項目を TaskCreate で登録し、各ステップ完了時に TaskUpdate で状態を更新すること。タスク化せずに作業を開始することは禁止
- **`log.Fatal*` 禁止** (`logger.Error` + `os.Exit(1)`、M0.3 既存パターン踏襲)
- **秘密鍵 / PEM フルダンプ / RSA modulus / exponent はログに出力禁止** (F-18 redact)
- **既存の M0.3 起動シーケンスを壊さない** (logger / config / db.Open / AssertClean の順序とエラーハンドリング維持)

## 作業ステップ (この順序で実行すること)

### ステップ 1: apperror パッケージへの code 追加

1. テスト先 (`core/internal/apperror/apperror_test.go` に追加):
   - `apperror.CodeEndpointNotImplemented = "ENDPOINT_NOT_IMPLEMENTED"` 定数追加
   - `apperror.New(CodeEndpointNotImplemented, "...")` で作成可能 (既存 `New` 関数を使う)
2. `core/internal/apperror/apperror.go` に定数追加 (もしくは別ファイル `code.go` 等、規約に従う):

   ```go
   const CodeEndpointNotImplemented = "ENDPOINT_NOT_IMPLEMENTED"
   ```

   注: notimpl handler は本 code を直接使わない (レスポンス body は OIDC snake_case `endpoint_not_implemented` を notimpl 内で固定文字列出力、論点 #8)。本 code は内部規約遵守 (将来 `apperror.New` 経由で構造化エラー化したいときの予約) として定義する。

3. `lint` & `test` パス
4. **Codex レビュー実行**
5. 指摘対応 → 次のステップ

### ステップ 2: server.go への route 登録

1. テスト先 (`core/internal/server/server_test.go` に追加):
   - `server.New(cfg, logger, pool, keystore)` シグネチャに `keystore.KeySet` 引数を追加
   - chi router に `/.well-known/openid-configuration` (GET, discovery handler) が登録される
   - chi router に `/jwks` (GET, jwks handler) が登録される
   - chi router に `/authorize` (GET), `/token` (POST), `/userinfo` (GET) が notimpl handler で登録される
   - middleware D1 順序 (`request_id → access_log → recover → handler`) が全 route に適用される (M0.2 既存 + 新規 route 全件)
   - `/.well-known/openid-configuration` と `/jwks` は **認証不要**、middleware も既存 D1 のみ (追加なし)
2. 実装 (`core/internal/server/server.go`):
   ```go
   func New(cfg *config.Config, l *logger.Logger, pool *pgxpool.Pool, ks keystore.KeySet) (*http.Server, error) {
       r := chi.NewRouter()
       r.Use(middleware.RequestID)
       r.Use(middleware.AccessLog(l))
       r.Use(middleware.Recover(l))
       // 既存 health
       r.Get("/health/live", healthLiveHandler)
       r.Get("/health/ready", healthReadyHandler(pool))
       // OIDC Discovery
       discoveryH, err := discovery.New(cfg.OIDC)
       if err != nil { return nil, err }
       r.Get("/.well-known/openid-configuration", discoveryH.ServeHTTP)
       // JWKS
       jwksH, err := jwks.New(ks, cfg.OIDC.JWKSMaxAge)
       if err != nil { return nil, err }
       r.Get("/jwks", jwksH.ServeHTTP)
       // notimpl 503 stub
       r.Get("/authorize", notimpl.Handler("M1.2"))
       r.Post("/token", notimpl.Handler("M1.3"))
       r.Get("/userinfo", notimpl.Handler("M1.4"))
       return &http.Server{Addr: fmt.Sprintf(":%d", cfg.Port), Handler: r}, nil
   }
   ```
3. `lint` & `test` パス
4. **Codex レビュー実行**
5. 指摘対応 → 次のステップ

### ステップ 3: main.go の起動シーケンス統合

1. テスト先 (`core/cmd/core/main_test.go` に追加):
   - `CORE_ENV=prod` + `CORE_OIDC_KEY_FILE` 未設定 + `CORE_OIDC_DEV_GENERATE_KEY` 未設定 → `run()` が exit code 1 (起動失敗、F-9, F-20-c)
   - `CORE_ENV=dev` + `CORE_OIDC_DEV_GENERATE_KEY=1` で起動成功 (DB / migrations は testutil/dbtest 経由で実 PostgreSQL を使う、M0.3 統合テストパターン踏襲)
   - 起動成功時、INFO ログに `kid` / `alg=RS256` / `source=generated` (or `file`) / `env=dev` フィールド (F-18, 観測性)
   - `CORE_OIDC_DEV_GENERATE_KEY=1` 起動時に WARN ログ「dev 鍵生成モード = 単一 Pod 専用、複数 Pod 環境では CORE_OIDC_KEY_FILE で共有 Secret を使え」(F-8)
   - 1024 bit 鍵をロードした場合 WARN ログ「鍵長 1024 bit は RS256 として弱い、2048 bit 以上を推奨」(P1 keystore で `BitLen()` 提供済、main 側で判定)
2. 実装 (`core/cmd/core/main.go`):
   - `bootstrap` の戻り値 `cfg` を経由して `keystore.Init(ctx, cfg.OIDC, l)` を呼び、`KeySet` と `Source` を取得
   - 失敗時は `l.Error(ctx, "鍵管理層の初期化に失敗しました", err)` + `return exitError`
   - 成功時は `l.Info(ctx, "起動鍵情報", slog.String("kid", kp.Kid), slog.String("alg", kp.Alg), slog.String("source", source.String()), slog.String("env", string(cfg.Env)))`
   - `source == SourceGenerated` の場合、`l.Warn(...)` を出す
   - `kp.BitLen() < 2048` の場合、別の `l.Warn(...)` を出す
   - `server.New(cfg, l, pool, ks)` に `ks` を渡す
3. `lint` & `test` パス
4. **Codex レビュー実行**
5. 指摘対応 → 次のステップ

### ステップ 4: 統合テスト (env 切替 / 起動失敗 / Discovery + JWKS 200 / kid 一致)

1. テスト先 (`core/cmd/core/integration_test.go` 新規 or 既存に追加、`testutil/dbtest` 経由で実 PostgreSQL):
   - **起動失敗統合**: `CORE_ENV=prod` + 鍵 env 未設定 → `run()` exit 1
   - **正常起動 + Discovery 200**: `CORE_ENV=dev` + `CORE_OIDC_DEV_GENERATE_KEY=1` で起動 → `httptest` 経由で `GET /.well-known/openid-configuration` → 200 + 期待 JSON
   - **正常起動 + JWKS 200**: 同上で `GET /jwks` → 200 + JWKS 構造体パース可能
   - **kid 一致**: Discovery メタデータの `jwks_uri` を取得 → 同じ起動の `/jwks` で得た kid と、起動 INFO ログの kid フィールドが一致
   - **503 stub**: `GET /authorize` / `POST /token` / `GET /userinfo` → 503 + body
   - **middleware D1 適用**: 全 route で `X-Request-Id` レスポンスヘッダ (M0.2 既存仕様、要確認) または access_log にエントリーが出る
2. 実装: 起動経路は `run(stderr)` 関数を使う (M0.3 で既に testable に切り出し済)、env は `t.Setenv` で制御
3. `lint` & `test` + `make -C core test-integration` パス
4. **Codex レビュー実行**
5. 指摘対応 → 次のステップ

### ステップ 5: docs/context/ 更新 (規約書 + registry + patterns + testing/backend)

1. `docs/context/backend/conventions.md` に「OIDC OP 規約」節を新設 (F-19、計 8 項目):
   1. issuer URL 規約 (https 必須 + dev 例外、末尾スラッシュ strip)
   2. 環境変数一覧 (`CORE_OIDC_*` / `CORE_ENV`)
   3. 鍵保管方式 (本番 = K8s Secret マウント / dev = devkeygen) + kid と fingerprint の関係 (F-11 / F-18 同値) + 鍵長ガイダンス (任意 bit 受け入れ、推奨 2048-4096) + **kid は RFC 7638 thumbprint 非準拠**、ログ表記は「kid」/「fingerprint」、「thumbprint」は使わない
   4. dev 鍵生成モードの単一 Pod 制約 (Helm/manifest 側で `replicas: 1` 強制、実 sample は M1.5)
   5. 開発者向け運用手順 (`make dev-keygen` / docker compose / make run)
   6. M1.1 の鍵更新運用制約 (F-24: Pod 全停止 → 起動) と M2.x の overlap window 予告 (F-22)
   7. 鍵フォーマット受け入れポリシー (PKCS#8 のみ、PKCS#1 / encrypted PEM はエラー、1024 bit 以下は WARN)
   8. JWKS 出力契約 (jwx バージョン go.mod 固定、`alg=RS256` / `kty=RSA` / `use=sig` 明示、private 成分非出力、golden / 契約テスト)

2. `docs/context/backend/registry.md` 更新:
   - パッケージ追加: `internal/keystore/`, `internal/oidc/discovery/`, `internal/oidc/jwks/`, `internal/oidc/notimpl/`
   - CLI 追加: `cmd/devkeygen/`
   - 環境変数追加: `CORE_ENV`, `CORE_OIDC_ISSUER`, `CORE_OIDC_KEY_FILE`, `CORE_OIDC_DEV_GENERATE_KEY`, `CORE_OIDC_KEY_ID`, `CORE_OIDC_JWKS_MAX_AGE`, `CORE_OIDC_DISCOVERY_MAX_AGE`, `CORE_OIDC_AUTHORIZATION_ENDPOINT`, `CORE_OIDC_TOKEN_ENDPOINT`, `CORE_OIDC_USERINFO_ENDPOINT`, `CORE_OIDC_JWKS_URI`
   - apperror code 追加: `ENDPOINT_NOT_IMPLEMENTED`

3. `docs/context/backend/patterns.md` 更新:
   - 起動時生成モード = メモリ保持限定パターン
   - 公開エンドポイント (`/.well-known/*` / `/jwks`) の middleware チェーン適用方針 (D1 順序踏襲)
   - 503 stub による未実装 endpoint のフォワード互換 (F-23、handler 差し替え動線)

4. `docs/context/testing/backend.md` 更新:
   - env 切替テストパターン (`CORE_ENV=prod` 起動失敗の検証手法)
   - ContractTest テーブル駆動例 (Q8 の 5 ケース)
   - golden ファイルテストパターン (jwx バージョン固定 + 差分レビュー必須)
   - 鍵長透過テストパターン

5. `core/README.md` 加筆 (or 新規):
   - dev 起動手順 (`make dev-keygen` → `docker compose up -d postgres` → `CORE_ENV=dev CORE_OIDC_ISSUER=http://localhost:8080 CORE_OIDC_KEY_FILE=./dev-keys/signing.pem make -C core run`)
   - Discovery / JWKS の curl 確認例

6. **Codex レビュー実行** (context 更新 + README 全体)
7. 指摘対応 → 次のステップ

### ステップ最終: 全体テスト + カバレッジ確認 + PR 作成

1. `make -C core lint test test-integration test-cover` 緑
2. CI workflow が緑であることを GitHub Actions で確認 (M0.3 で導入済の `core/test-integration` ワークフロー)
3. PR 作成 → `/pr-codex-review {番号}` で最終 Codex レビュー → CRITICAL=0/HIGH=0/MEDIUM<3 で main にマージ
4. 親 Issue #32 に「M1.1 完了」コメント (実装 4 件すべて完了したことを記載、ユーザーが手動 close)

## 実装コンテキスト

以下のファイルを読み取ってから実装を開始すること:

```
CONTEXT_DIR="docs/context"
```

- `${CONTEXT_DIR}/app/architecture.md`
- `${CONTEXT_DIR}/backend/conventions.md` (本タスクで OIDC OP 規約節を追記)
- `${CONTEXT_DIR}/backend/patterns.md` (本タスクで OIDC パターンを追記)
- `${CONTEXT_DIR}/backend/registry.md` (本タスクで `CORE_OIDC_*` env / パッケージ / apperror code を追記)
- `${CONTEXT_DIR}/testing/backend.md` (本タスクで env 切替 / ContractTest / golden / 鍵長透過テストパターンを追記)

設計書: `docs/specs/32/index.md` (特に「既存実装からの統合点」「既存資料からの差分」「フロー図 / シーケンス図 → 起動シーケンス」「テスト観点」)

要求文書: `docs/requirements/32/index.md` (F-8, F-9, F-15, F-18, F-19, F-20)

適用範囲: `core/cmd/core/`, `core/internal/server/`, `core/internal/apperror/`, `docs/context/`, `core/README.md` (新規 or 加筆)

## 前提条件

- **P1, P2, P3 すべて完了** (config + keystore + devkeygen + Discovery + JWKS + notimpl がマージ済)
- 本タスクは M1.1 の最終配線 + ドキュメント整備で、完了をもって M1.1 マイルストーン達成
- 後続: M1.2 (`/authorize` 実装、本タスクで registered した notimpl を本実装に置き換え)

## 不明点ハンドリング

- 矛盾・欠落・未定義がある場合は作業を止める
- 勝手な推測で実装を進めない
- 質問時は: 止まっている作業単位 / 判断が必要な論点 / 選択肢を整理して提示
- 例: 「`server.New` シグネチャ変更で M0.2/M0.3 既存テストがどこまで影響受けるか」「kid のログ出力方式 (フィールド名 `kid` で固定すべきか `fingerprint` も同時に出すか)」等で迷ったらユーザー確認

## タスク境界

### 実装する範囲

- `core/internal/apperror/` への `CodeEndpointNotImplemented` 定数追加 + テスト
- `core/internal/server/server.go` への OIDC route 登録 + `keystore.KeySet` 引数追加 + テスト
- `core/cmd/core/main.go` への keystore 初期化挿入 + 起動 INFO/WARN ログ + テスト
- `core/cmd/core/integration_test.go` 統合テスト (env 切替 / 起動失敗 / Discovery+JWKS 200 / kid 一致 / 503 stub)
- `docs/context/backend/conventions.md` への OIDC OP 規約節追記 (8 項目)
- `docs/context/backend/registry.md` 更新 (パッケージ / env / apperror code)
- `docs/context/backend/patterns.md` 更新 (起動時生成モード / 公開 endpoint middleware / 503 stub フォワード互換)
- `docs/context/testing/backend.md` 更新 (env 切替 / ContractTest / golden / 鍵長透過)
- `core/README.md` 加筆 (dev 起動手順 + curl 確認例)

### 実装しない範囲 (後続マイルストーン)

- M1.2 `/authorize` 本実装 (notimpl 置き換え)
- M1.3 `/token` 本実装
- M1.4 `/userinfo` 本実装
- M1.5 K8s manifest sample / Helm chart sample / E2E with go-react RP
- M2.x 鍵 rotation API、`multiKeySet` (複数鍵保持)、overlap window 実装
- M4.x id-core が RP として上流 IdP と連携する経路

## 設計仕様 (設計書から本タスク該当箇所を抜粋)

### 起動シーケンス (F-15、論点 #9)

```
bootstrap (config → logger → event_id)
  → ctx (WithEventID)
  → db.Open (M0.3)
  → AssertClean (M0.3)
  → keystore.Init  ★ M1.1 で新規挿入
  → INFO 起動鍵情報 (kid / alg / source / env)
  → WARN dev 鍵生成モード (source=generated 時のみ)
  → WARN 鍵長 < 2048 (BitLen() で判定、該当時のみ)
  → server.New (cfg, logger, pool, keystore)
  → ListenAndServe
```

各ステップで失敗 → `logger.Error` + `os.Exit(1)` (M0.3 既存パターン踏襲、`log.Fatal*` 不使用)。

### route 登録 (server.go)

| Method | Path                                | Handler                 | 認証              |
| ------ | ----------------------------------- | ----------------------- | ----------------- |
| GET    | `/health/live`                      | (M0.3 既存)             | 不要              |
| GET    | `/health/ready`                     | (M0.3 既存)             | 不要              |
| GET    | `/.well-known/openid-configuration` | discovery.Handler       | 不要 (公開、F-16) |
| GET    | `/jwks`                             | jwks.Handler            | 不要 (公開、F-16) |
| GET    | `/authorize`                        | notimpl.Handler("M1.2") | 不要 (公開、stub) |
| POST   | `/token`                            | notimpl.Handler("M1.3") | 不要 (公開、stub) |
| GET    | `/userinfo`                         | notimpl.Handler("M1.4") | 不要 (公開、stub) |

middleware: D1 順序 (`request_id → access_log → recover → handler`、M0.2 確定)、全 route に適用。

### 起動ログ仕様 (F-18、観測性)

INFO (常時、起動成功時 1 回):

```json
{
  "level": "INFO",
  "msg": "起動鍵情報",
  "kid": "abcd1234efgh5678ijkl9012",
  "alg": "RS256",
  "source": "generated",
  "env": "dev",
  "event_id": "<UUID v7>"
}
```

WARN (dev 鍵生成モード時のみ、F-8 / Q5):

```json
{
  "level": "WARN",
  "msg": "dev 鍵生成モード = 単一 Pod 専用、複数 Pod 環境では CORE_OIDC_KEY_FILE で共有 Secret を使え"
}
```

WARN (鍵長 < 2048 のとき、論点 #10 Codex MEDIUM 2):

```json
{
  "level": "WARN",
  "msg": "鍵長 1024 bit は RS256 として弱い、2048 bit 以上を推奨",
  "bit_len": 1024
}
```

**禁止事項** (F-18 redact): 秘密鍵 / PEM フルダンプ / RSA modulus (`n`) / exponent (`e`) のログ出力禁止。

## テスト観点 (本タスク該当のみ)

### server 単体

- `server.New` に `keystore.KeySet` 引数追加
- 全 OIDC route が正しく登録される
- middleware D1 順序が全 route に適用される

### main 単体

- `CORE_ENV=prod` + 鍵未設定で `run()` が exit 1 (F-20-c)
- `CORE_ENV=dev` + `CORE_OIDC_DEV_GENERATE_KEY=1` で起動成功
- 起動 INFO ログに kid / alg / source / env フィールド
- dev 鍵生成モード時 WARN ログ
- 鍵長 < 2048 で WARN ログ

### 統合テスト (testutil/dbtest 経由)

- 上記起動失敗 / 起動成功シナリオを実 PostgreSQL 接続で実行
- 起動成功後、`httptest` + chi router で Discovery / JWKS / 503 stub の HTTP レスポンスを検証
- Discovery の `jwks_uri` URL → `/jwks` の kid → 起動ログの kid の三者一致

### context 更新の妥当性

- conventions.md OIDC OP 規約節に 8 項目すべて記載
- registry.md にパッケージ / env / apperror code 全追加
- patterns.md, testing/backend.md, README.md の追記内容が設計書 / 要求文書と整合

## Codex レビューコマンド (各ステップで使用)

```bash
codex exec --full-auto -c model_reasoning_effort="medium" \
  "まず以下の context ファイルを読み取れ:
   - docs/context/app/architecture.md
   - docs/context/backend/conventions.md
   - docs/context/backend/patterns.md
   - docs/context/backend/registry.md
   - docs/context/testing/backend.md
   設計書: docs/specs/32/index.md (特に既存実装からの統合点 / 起動シーケンス図 / 既存資料からの差分)
   要求文書: docs/requirements/32/index.md (F-8, F-9, F-15, F-18, F-19, F-20)

   その上で git diff をレビューせよ。

   Check (本タスクは main + server 統合 + 統合テスト + context 更新):
   1) TDD compliance (テスト先行 / 統合テストカバレッジ網羅)
   2) 起動シーケンスが M0.3 既存順序を壊していない (logger / config / db.Open / AssertClean を維持)
   3) keystore.Init の挿入位置 (db.Open 後、server.New 前) と失敗時 logger.Error + os.Exit(1)
   4) server.New シグネチャ変更が既存テストを壊していない (M0.2/M0.3 既存テストの修正を含む)
   5) 全 OIDC route が middleware D1 順序を通る
   6) 起動 INFO ログに kid / alg=RS256 / source / env が出力される (F-18 観測性)
   7) WARN ログ条件 (dev 鍵生成モード時 / 鍵長 < 2048 時) が正しく発火する
   8) 秘密鍵 / PEM / RSA n,e がログに出力されないこと (F-18 redact)
   9) 統合テスト (env 切替 / Discovery+JWKS+kid 一致) が実 PostgreSQL で実行される (testutil/dbtest 経由)
   10) docs/context/ 更新の網羅性 (OIDC OP 規約 8 項目 / registry / patterns / testing 全件)
   11) README 加筆の正確性 (dev 起動手順が動作する)
   12) log.Fatal* 不使用、UUID v7 規約、Co-Authored-By trailer 不使用
   13) 探索禁止違反がないか

   Report issues as CRITICAL/HIGH/MEDIUM/LOW in Japanese. Last section must be ## Summary with counts and gate verdict."
```

## 完了条件

- [ ] ステップ 1: apperror に `CodeEndpointNotImplemented` 追加 + テスト 完了 + Codex ゲート PASS
- [ ] ステップ 2: server.go に OIDC route 登録 + `keystore.KeySet` 引数追加 + テスト 完了 + Codex ゲート PASS
- [ ] ステップ 3: main.go に keystore 初期化 + 起動 INFO/WARN ログ + テスト 完了 + Codex ゲート PASS
- [ ] ステップ 4: 統合テスト (env 切替 / Discovery+JWKS 200 / kid 一致 / 503 stub) 完了 + Codex ゲート PASS
- [ ] ステップ 5: docs/context/ + README 全更新 完了 + Codex ゲート PASS
- [ ] `make -C core lint test test-integration test-cover` 緑
- [ ] CI workflow (GitHub Actions) 緑
- [ ] PR 作成 (`/pr-codex-review {番号}` で最終 Codex レビュー、CRITICAL=0/HIGH=0/MEDIUM<3 で main にマージ)
- [ ] 親 Issue #32 に「M1.1 完了」コメントを投稿、ユーザーが手動 close
- [ ] 未解決の仕様質問が残っていない
