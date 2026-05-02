# P3: ログスキーマ契約テスト + context / README / Makefile 更新

- 対応 Issue: #14 (【実装】M0.2 ログスキーマ契約テスト + context / README / Makefile 更新)
- 元要求: #7
- 親設計書: `docs/specs/7/index.md`
- 先行プロンプト: `P1_01_core_logger_apperror.md` / `P2_01_core_middleware_server_main.md`
- マイルストーン: M0.2 (Phase 0: スパイク)

## 絶対ルール

- **コードベース探索禁止**: Grep, Glob, Read による既存コードの探索的な読み取りを行わない
- **Explore エージェント起動禁止**: サブエージェントによるコードベース調査を行わない
- 実装に必要な情報は全て、以下の context ファイルとこのプロンプト内に記載済み
- context に記載がなく実装判断できない場合は、探索ではなく**ユーザーに質問**すること
- 変更対象ファイルの直接の Read / Edit のみ許可 (探索目的の Read は禁止)
- **Codex レビューをスキップしない**: 各作業ステップに Codex レビューが含まれている。レビューを受けずに次のステップに進むことは禁止
- **完了条件のタスク化**: 作業開始前に「完了条件」セクションの各項目を TaskCreate で登録し、各ステップ完了時に TaskUpdate で状態を更新すること。タスク化せずに作業を開始することは禁止

### プロジェクト固有の禁止事項 (恒久ルール)

- **UUID v4 禁止** / **`log.Fatal*` 禁止** / **`time.Local = time.UTC` 禁止** / **redact 部分一致禁止** / **Domain 層ログ禁止** (詳細は P1 / P2 参照)
- **context 4 ファイルへの記述で「アーカイブ」「特定リポジトリ名」「社内固有事情」を書かない**: 公開リポジトリ前提のテンプレート扱い

### コミット時の禁止事項

- コミットメッセージに `Co-Authored-By:` trailer を入れない
- main ブランチへ直接 push しない (feature ブランチ + PR + Codex レビュー必須)

## 作業ステップ (この順序で実行すること)

### ステップ 0: 前提セットアップ

1. Issue #12 (P1) と Issue #13 (P2) が両方マージ済みを確認 (`core/internal/{logger,apperror,middleware}/` が完成し、`server.go` / `main.go` / `health.go` が更新済み)
2. 作業ブランチ `feature/m0.2-impl-contract-context` を `main` から切る
3. `make -C core build && make -C core test && make -C core lint` がベースラインで pass

> **Codex レビュー対象外**: ステップ 0 はブランチ作成・前提確認のみで成果物変更を伴わない。レビューはステップ 1 以降で実施する。

### ステップ 1: F-16 ログスキーマ契約テスト

1. テストを先に書く: `core/internal/logger/contract_test.go` を新規作成し T-58〜T-62 を実装 (失敗確認)
2. 実装方針: テスト用 `bytes.Buffer` を writer に渡してロガーを初期化、HTTP 系と非 HTTP 系それぞれで 1 行ログ出力 → JSON decode → 必須フィールドの存在 + 型を `t.Fatalf` で検証
3. `make -C core lint test` 全件 pass
4. **Codex レビューを実行**
5. 指摘を対応してから次のステップへ

### ステップ 2: `core/Makefile` の lint で `log.Fatal` 検査追加

1. テストを先に書く: shell ベースの試験 (CI 上の grep 結果) で簡単に検証する。Makefile の `lint` target を実行した際に、`core/` 配下に `log.Fatal*` を**新規追加**したファイルがあれば lint failure になることを確認
2. 実装方針: `lint` target に以下を追加 (実装者は具体的なシェル構文を整える)

   ```makefile
   .PHONY: lint
   lint:
   	@echo "==> go vet"
   	@go vet ./...
   	@echo "==> log.Fatal check"
   	@if grep -rn 'log\.Fatal' . --include='*.go' | grep -v '_test\.go'; then \
   		echo "ERROR: log.Fatal* is forbidden in core/. Use logger.Error + os.Exit(1) instead."; \
   		exit 1; \
   	fi
   ```

3. `make -C core lint` が pass することを確認
4. 試験的に `core/cmd/core/main.go` に `log.Fatalf("test")` を一時追加して `make -C core lint` が **失敗**することを確認 (確認後に元に戻す)
5. **Codex レビューを実行**
6. 指摘を対応してから次のステップへ

### ステップ 3: `core/README.md` にログ・エラー規約の入口段落を追加

1. 既存の `core/README.md` に以下のような段落を追記 (既存の構造を尊重しながら)
2. 内容:
   - ログ・エラー規約の概要 (1〜2 段落)
   - `CORE_LOG_FORMAT` 環境変数の説明 (デフォルト `json`、開発時は `text`)
   - `docs/context/backend/conventions.md` のロギング・テレメトリ節とエラーハンドリング節へのリンク
   - panic 時のレスポンス形式 (`{ code: "INTERNAL_ERROR", message: "...", request_id: "..." }`) の言及
3. **Codex レビューを実行**
4. 指摘を対応してから次のステップへ

### ステップ 4-7: `docs/context/` 4 ファイル更新 (1 ステップで実施)

context 4 ファイルは互いに整合性を取る必要があるため、**1 つのコミット単位**として実施し、末尾でまとめて Codex レビューを実行する。各ファイルの更新内容は以下のサブセクション (4-1 〜 4-4) で個別に指示する。

サブステップを順次実施した後、以下を必ず実行する:

1. `make -C core lint test` が pass (context 更新は core/ には影響しないが、既存テストを壊していないか確認)
2. `npx prettier --check docs/context/backend/*.md docs/context/testing/backend.md` で整形確認 (失敗時は `--write` で整形)
3. **Codex レビューを実行** (コマンドは末尾、context 4 ファイルの整合性検証を含める)
4. 指摘を対応してから次のステップへ

#### サブステップ 4-1: `docs/context/backend/conventions.md` 更新

1. 「ロギング・テレメトリ」節の M0.1 暫定記述を**詳細化**:
   - ロガー実装: `log/slog` (Go 標準)
   - フォーマット切替: `CORE_LOG_FORMAT=json|text` (デフォルト `json`)
   - 一意 ID 生成: UUID v7 (`github.com/google/uuid` v1.6+ の `uuid.NewV7()`、v4 は使用禁止)
   - `time` フィールド: RFC3339Nano UTC、`Z` suffix 強制 (`slog` の `ReplaceAttr` で UTC 変換、`time.Local = time.UTC` は禁止)
   - ログレベル使い分け表 (DEBUG=本番無効 / INFO=業務イベント / WARN=4xx / ERROR=5xx・panic)
   - HTTP 経路 / 非 HTTP 経路の必須フィールド一覧
   - ログ出力失敗時の挙動 (stderr フォールバック + atomic drop counter、リクエスト処理は継続)
   - ログメッセージ言語: 日本語
2. 「エラーハンドリング」節を**新設**:
   - `internal/apperror/` パッケージ
   - F-7 基本形 `{ code, message, details?, request_id }` の JSON 形式
   - `code` 命名規則: `SCREAMING_SNAKE_CASE`
   - `details` の型制約: object / array のみ、シークレット禁止 (redact 連携)
   - redact 対象キー一覧 (Q8 完全リスト 16 キー + 6 ヘッダ、case-insensitive 完全一致、ネスト・配列再帰走査、`[REDACTED]` 固定値)
   - OIDC エンドポイント (M1.x 以降) の RFC 6749 / 6750 準拠方針
3. 「middleware 構成」節を**新設**:
   - D1 順序: `request_id` → `access_log` → `recover` → `handler`
   - 各 middleware の責務
   - `context.Context` 経由の `request_id` / `event_id` 伝播 (Domain 層は context から取得のみ)
4. 「環境変数」節 (registry.md と整合) に `CORE_LOG_FORMAT` を追加

#### サブステップ 4-2: `docs/context/backend/patterns.md` 更新

1. 「middleware チェーンパターン」節を**新設**: D1 順序の図とコード例 (`http.HandlerFunc` を wrap する実装サンプル)
2. 「context への ID 付与パターン」節を**新設**: `WithRequestID` / `RequestIDFrom` / `WithEventID` / `EventIDFrom` のコード例。Domain 層は context から取り出すのみ
3. 「redact パターン」節を**新設**: `internal/logger/redact.go` の deny-list 実装の最小サンプル (case-insensitive ヘッダ照合 / JSON 再帰走査 / `[REDACTED]` 固定置換)
4. 「エラーハンドリング」節を**置換**: M0.1 暫定の `fmt.Errorf` + `%w` ラップ記述を、`apperror.New(code, message)` / `WithDetails(...)` / `Wrap(cause)` パターンに置換
5. 「ログ出力失敗時のフォールバック」節を**新設**: stderr フォールバック + atomic drop counter のコード例

#### サブステップ 4-3: `docs/context/backend/registry.md` 更新

1. 「パッケージマッピング」表に以下を追加:
   - `core/internal/logger` (構造化ロガー、依存: `log/slog`, `github.com/google/uuid`)
   - `core/internal/apperror` (エラー型 + JSON シリアライザ)
   - `core/internal/middleware` (request_id / access_log / recover、依存: `internal/logger`, `internal/apperror`)
2. 「環境変数一覧」表に `CORE_LOG_FORMAT` を追加 (既定値 `json`、値は `json` または `text`、必須=任意)
3. 「エラーコード一覧」節を新設し、最低限 `INTERNAL_ERROR` (panic 時の固定 code、F-9 / F-10) を記載。他のコードは M1.x 以降で OIDC 標準コードと統合

#### サブステップ 4-4: `docs/context/testing/backend.md` 更新 (TBD → 最低限の埋め込み)

1. 「Go (id-core) のテスト」節を埋める:
   - 標準パターン: `httptest.NewRequest` + `mux.ServeHTTP` (M0.1 から踏襲)
   - 外部テストパッケージ命名 (`<pkg>_test`)
   - `t.Setenv` と `t.Parallel` の併用不可 (Go 仕様)
   - テーブル駆動テスト (redact deny-list で活用)
   - ログ buffer での検証パターン (`bytes.Buffer` を `slog` ハンドラに渡し、JSON decode で検証)
   - `grep -rn "log\.Fatal" core/` ガード (Makefile の lint で実行)
2. 「ログスキーマ契約テスト」節を新設: F-16 のフィールド存在 + 型検証パターン。HTTP 系・非 HTTP 系の 2 系統に分割。フィールド追加は許容、削除・型変更はテスト失敗の方針

### ステップ 5: 全体テスト + ベースライン確認 + 最終 Codex レビュー

1. `make -C core build && make -C core test && make -C core lint` 全件 pass
2. `grep -rn "log\.Fatal" core/` の出力が 0 件 (P2 で達成済み、本フェーズで Makefile lint がそれを保証)
3. CI 全 green
4. PR 作成 (`gh pr create` with `--assignee` + `--label`)
5. **`/pr-codex-review {PR 番号}` で Codex に PR 全体 (差分 + description) をレビューさせる**。これが本フェーズの最終 Codex レビュー (絶対ルール「Codex レビュー必須」を満たす全体検査)
6. ゲート通過 (CRITICAL=0 / HIGH=0 / MEDIUM<3) → ユーザー承認 → マージ

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

設計書: `docs/specs/7/index.md` (特に「既存資料からの差分」節を参照)

先行実装: `core/internal/{logger,apperror,middleware}/` および `core/internal/server/`、`core/cmd/core/main.go` (P1 / P2 で完成済み)

適用範囲:

- `core/internal/logger/contract_test.go` (新規)
- `docs/context/backend/{conventions,patterns,registry}.md` (既存修正)
- `docs/context/testing/backend.md` (既存修正、TBD → 最低限埋める)
- `core/Makefile` (既存修正、lint で grep 検査)
- `core/README.md` (既存修正、規約入口段落)

## 前提条件

- **Issue #12 (P1) と Issue #13 (P2) が両方マージ済み**: `core/internal/{logger,apperror,middleware}/` が完成し、`server.go` / `main.go` / `health.go` が更新済み
- M0.1 (Issue #1, #2) 完了済み
- 本 Issue (#14) のマージで M0.2 マイルストーンが完了する

## 不明点ハンドリング

- 矛盾・欠落・未定義がある場合は作業を止める
- 推測で実装を進めない
- 質問時は: 止まっている作業単位 / 判断が必要な論点 / 選択肢を整理
- 特に以下が判明した時点で**即停止してユーザーに質問**する:
  - F-16 契約テストでフィールド存在 + 型を JSON decode 後にどこまで厳密にチェックするか
  - Makefile の lint で `log.Fatal` 検査を実装する具体的な構文 (POSIX shell vs GNU make の互換性)
  - context ファイルの既存記述と本プロンプトの新規記述で表現が衝突する場合の優先順位

## タスク境界

### 本プロンプトで実装する範囲

- `core/internal/logger/contract_test.go` (新規、T-58〜T-62 の 5 ケース)
- `docs/context/backend/conventions.md` (ロギング・テレメトリ詳細化、エラーハンドリング新設、middleware 構成新設、環境変数追記)
- `docs/context/backend/patterns.md` (middleware チェーンパターン / context ID 付与 / redact パターン / エラーハンドリング置換 / フォールバックパターン)
- `docs/context/backend/registry.md` (パッケージマッピング 3 件追加、環境変数 1 件追加、エラーコード一覧新設)
- `docs/context/testing/backend.md` (Go テスト節埋め、ログスキーマ契約テスト節新設)
- `core/Makefile` (lint で `grep -rn "log\.Fatal"` 検査追加)
- `core/README.md` (ログ・エラー規約の入口段落追加)
- 上記のテストケース T-58〜T-62 = **5 ケース**

### 本プロンプトでは実装しない範囲

- `core/internal/{logger,apperror,middleware}/` のコア実装 → P1 / P2 で完了済み
- `core/internal/server/server.go` / `core/cmd/core/main.go` の修正 → P2 で完了済み

## 設計仕様

### 関連要件 (F-N) の引用

| ID   | 内容                                                                                                                                                                                                                                                                                            |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| F-12 | M0.1 由来の `log.Fatal*` を `core/` 配下から完全に排除する。完了条件は `grep -rn "log\.Fatal" core/` 出力が 0 件 (P2 で達成、本フェーズで Makefile lint が違反を CI で検知)                                                                                                                     |
| F-15 | `core/` で `make build && make test && make lint` が pass する                                                                                                                                                                                                                                  |
| F-16 | ログスキーマ契約テストが用意されている。検証対象は最低 2 系統に分割: (a) HTTP 経路系 — `time` / `level` / `msg` / `request_id` / `method` / `path` / `status` / `duration_ms` の存在と型 (最低 1 ケース)、(b) 非 HTTP 経路系 — `time` / `level` / `msg` / `event_id` の存在と型 (最低 1 ケース) |
| F-17 | 規約書が存在する。最低必須項目: ログフィールド定義 / エラー境界 / redact 対象キーの完全一覧と適用面 / ログレベル使い分けガイド / 開発者向け運用手順                                                                                                                                             |

### 関連論点 (Q-N) の決定値

| ID  | 決定                                                                                                                                                                                                   |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Q10 | 規約書の格納場所: **`docs/context/backend/conventions.md` のロギング・テレメトリ節を本スコープで詳細化 + エラーハンドリング節を新設して `apperror` 規約を記載** (新規ディレクトリ作成は回避、単一 SoT) |

### F-16 契約テストの実装方針 (詳細)

```go
// core/internal/logger/contract_test.go の構造例 (実装者がテーブル駆動で書く)

package logger_test

import (
    "bytes"
    "encoding/json"
    "testing"

    "github.com/mktkhr/id-core/core/internal/logger"
)

func TestContract_HTTPPathSchema(t *testing.T) {
    // T-58: HTTP 経路ログ (msg=access) のフィールド存在 + 型
    var buf bytes.Buffer
    l := logger.New(logger.FormatJSON, &buf)
    // ... access_log middleware が出すログを模した出力を作る
    // ... json.Unmarshal で map[string]any にデコード
    // ... 必須フィールドが存在するか + 型が一致するか検証
}

func TestContract_NonHTTPPathSchema(t *testing.T) {
    // T-59: 非 HTTP 経路ログのフィールド存在 + 型
    // ... event_id を持つログを出して同様に検証
}

func TestContract_FieldAdditionAllowed(t *testing.T) {
    // T-60: 追加フィールドが含まれてもテストは失敗しない
}

func TestContract_FieldRemovalFails(t *testing.T) {
    // T-61: 必須フィールドが欠けるとテスト失敗
    // (実装者が「フィールド欠落の状態」を意図的に作ってテスト)
}

func TestContract_FieldTypeChangeFails(t *testing.T) {
    // T-62: 型不一致でテスト失敗
}
```

### `core/README.md` 追記の具体案

既存 README の構造を尊重しつつ、新規節 (例: 「ログ・エラー規約」) として:

```markdown
## ログ・エラー規約

`core/` のログとエラーレスポンスは構造化規約に従う:

- **ログ**: `log/slog` ベースの JSON Lines (本番) / Text (開発、`CORE_LOG_FORMAT=text` で切替)
- **request_id**: 全 HTTP リクエストに UUID v7 で発番、レスポンスヘッダ `X-Request-Id` で返却
- **エラーレスポンス**: `internal/apperror/` パッケージの基本形 `{ code, message, details?, request_id }`
- **panic 時**: 固定メッセージ + `request_id` のみ返し、スタックトレースは内部ログにのみ記録

詳細な規約は [`docs/context/backend/conventions.md`](../docs/context/backend/conventions.md) のロギング・テレメトリ節とエラーハンドリング節を参照。

### 環境変数

| Key               | 既定値 | 値の例               | 説明                          |
| ----------------- | ------ | -------------------- | ----------------------------- |
| `CORE_PORT`       | `8080` | `1`〜`65535` の整数  | HTTP サーバーのリッスンポート |
| `CORE_LOG_FORMAT` | `json` | `json` または `text` | ログ出力フォーマットの切替    |
```

### `core/Makefile` 追記の具体案

```makefile
.PHONY: lint
lint:
	@echo "==> go vet"
	@go vet ./...
	@echo "==> log.Fatal* check (project policy: forbidden in core/)"
	@if grep -rn --include='*.go' --exclude='*_test.go' 'log\.Fatal' . ; then \
		echo "ERROR: log.Fatal* is forbidden in core/. Use logger.Error + os.Exit(1) instead."; \
		exit 1; \
	fi
```

(注: `--include` / `--exclude` は GNU grep の機能。POSIX 互換が必要なら `find ... | xargs grep` パターンに変える)

## テスト観点

### F-16 ログスキーマ契約テスト (T-58〜T-62)

| #    | カテゴリ | 観点                                                                 | 期待                                                                                                                                                                                  | 関連      |
| ---- | -------- | -------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| T-58 | 契約     | HTTP 経路ログ (`msg=access`) のフィールド存在 + 型                   | 必須: `time` (string, RFC3339Nano UTC) / `level` (string) / `msg` (string) / `request_id` (string) / `method` (string) / `path` (string) / `status` (number) / `duration_ms` (number) | F-16      |
| T-59 | 契約     | 非 HTTP 経路ログ (起動 / signal handler / job) のフィールド存在 + 型 | 必須: `time` (string) / `level` (string) / `msg` (string) / `event_id` (string)                                                                                                       | F-16, F-4 |
| T-60 | 契約     | フィールド追加は許容 (将来の拡張)                                    | 追加フィールドが含まれてもテスト失敗しない                                                                                                                                            | F-16      |
| T-61 | 契約     | フィールド削除は失敗 (破壊的変更検知)                                | 必須フィールドが欠けると `t.Errorf` で失敗                                                                                                                                            | F-16      |
| T-62 | 契約     | フィールド型変更は失敗 (例: `status` が string になる等)             | 型不一致でテスト失敗                                                                                                                                                                  | F-16      |

## Codex レビューコマンド (各ステップで使用)

```bash
CONTEXT_DIR="docs/context"

codex exec --sandbox read-only "Review the staged + unstaged diff (Go + Makefile + Markdown).

Context to read first:
- ${CONTEXT_DIR}/app/architecture.md
- ${CONTEXT_DIR}/backend/conventions.md
- ${CONTEXT_DIR}/backend/patterns.md
- ${CONTEXT_DIR}/backend/registry.md
- ${CONTEXT_DIR}/testing/backend.md
- docs/specs/7/index.md (本フェーズ設計書、特に「既存資料からの差分」節)
- docs/specs/7/prompts/P3_01_core_contract_context_makefile.md (本プロンプト)

Then review the current diff (use git diff). Check:
1) F-16 contract test: HTTP 系 + 非 HTTP 系の両方をカバー、フィールド存在 + 型を厳密に検証
2) Makefile lint で log.Fatal 検査が機能する (新規追加で lint failure する)
3) README の規約入口段落が conventions.md と registry.md にリンクし、内容矛盾がない
4) context 4 ファイルの整合: conventions.md (規約本文) / patterns.md (実装サンプル) / registry.md (パッケージ・環境変数・エラーコード) / testing/backend.md (テストパターン) が一貫している
5) M0.1 の既存記述 (TBD 等) を勝手に消していないか / 必要な節だけを更新しているか
6) アーカイブ参照や特定リポジトリ名の混入がないか (公開リポジトリ前提)
7) 設計書 docs/specs/7/index.md の「既存資料からの差分」節の指示と齟齬がないか

Report issues as CRITICAL/HIGH/MEDIUM/LOW in Japanese with file:line references."
```

## 完了条件

- [ ] Issue #12 (P1) と Issue #13 (P2) のマージ済みを確認
- [ ] 作業ブランチ `feature/m0.2-impl-contract-context` を作成
- [ ] `core/internal/logger/contract_test.go` 実装、T-58〜T-62 全件 pass
- [ ] `core/Makefile` の `lint` target に `grep -rn "log\.Fatal" core/` 検査を追加、既存テスト pass を維持
- [ ] `core/Makefile` の lint が `log.Fatal` 新規追加を検知する (試験的に追加して `make -C core lint` 失敗を確認 → 戻す)
- [ ] `core/README.md` にログ・エラー規約の入口段落を追加
- [ ] `docs/context/backend/conventions.md` を更新 (ロギング・テレメトリ詳細化、エラーハンドリング節新設、middleware 構成節新設、環境変数追記)
- [ ] `docs/context/backend/patterns.md` を更新 (middleware チェーン / context ID 付与 / redact / エラーハンドリング / フォールバック)
- [ ] `docs/context/backend/registry.md` を更新 (パッケージマッピング、環境変数、エラーコード)
- [ ] `docs/context/testing/backend.md` を最低限の内容で埋める
- [ ] `make -C core build && make -C core test && make -C core lint` 全件 pass
- [ ] `grep -rn "log\.Fatal" core/` の出力が 0 件 (P2 で達成済み、本フェーズで保護)
- [ ] 各ステップで Codex レビューを実施、CRITICAL=0 / HIGH=0 / MEDIUM<3
- [ ] PR 作成 (`gh pr create` with `--assignee` + `--label "種別:実装" --label "対象:基盤"`)
- [ ] `/pr-codex-review {番号}` でゲート通過
- [ ] PR の Test plan を実機確認して `[x]` に書き換え
- [ ] PR をマージ (Issue #14 が自動 close)
- [ ] 親 Issue #7 の task list で #14 にチェックが付くことを確認
- [ ] **M0.2 マイルストーンを close** (全実装 Issue #12 / #13 / #14 の close 後にユーザーが実施、または提案)
- [ ] 親要求 Issue #7 を close (M0.2 完了確認後にユーザーが実施)
