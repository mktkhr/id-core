# 実装プロンプト生成ルール

## 実装ツール選択 (実行時に確認)

`AskUserQuestion` で実装セッションに使うツールを確認する。

| ツール                    | モデル例              | 分割方針                     | 1 プロンプトの目安                     |
| ------------------------- | --------------------- | ---------------------------- | -------------------------------------- |
| **Codex CLI** (推奨)      | gpt-5 系 / codex-5 系 | 同一リソース・同一画面は統合 | 最大 6EP、画面 1 枚全体 OK、300-500 行 |
| **Claude Code** (Opus 1M) | claude-opus-4-7       | 同上                         | 同上                                   |
| **Claude Code** (軽量)    | claude-haiku-4-5      | 細粒度分割                   | 最大 1-2EP、120-180 行                 |

デフォルト: **Codex CLI**。`which codex` で判定し、未インストールなら Haiku を提案。

## フェーズ分割ルール

### 統合の判断基準

モデル容量が許す限り **1 プロンプトに統合する**:

- 同一リソースの CRUD
- 同一エンティティへの拡張
- 同一画面の全セクション

### 分割を維持すべき境界

- DDL / マイグレーション と アプリケーションコード
- バックエンド と フロントエンド
- フロントエンド と E2E テスト
- `core/` (id-core 本体) と `examples/...` (動作確認用サンプル)

### 依存関係

| 依存関係                                             | フェーズ分割          |
| ---------------------------------------------------- | --------------------- |
| DDL → 全バックエンド                                 | DDL は最初のフェーズ  |
| バックエンド API → フロントエンド                    | フロントは API の後   |
| 全画面 → E2E テスト                                  | E2E は最後            |
| 同じテーブルを参照する API 同士                      | 並列 OK               |
| `core/` 認可エンドポイント → `examples/...` 動作確認 | サンプルは core/ の後 |

## コンテキスト参照ルール (最重要)

### 探索禁止の原則

**「既存コードを参照して」「既存実装を確認して」等の探索誘発指示を絶対に書かない。**

- ❌ 「既存の `internal/features/` 配下の実装パターンを参照して構造を踏襲」
- ❌ 「ログイン画面の既存実装を確認すること」
- ✅ 「`${CONTEXT_DIR}/backend/patterns.md` のパターンに従って実装する」

### context/ パス参照

`${CONTEXT_DIR}` を定義し、必要な context ファイルを列挙する。**内容は埋め込まない**。

```
CONTEXT_DIR="docs/context"
```

(リポジトリルートからの相対パス。`.rulesync/rules/path-resolution.md` 準拠)

### context/ 参照マッピング

| プロンプト種別     | 実装時に参照                                                                                      | Codex レビューに渡す  |
| ------------------ | ------------------------------------------------------------------------------------------------- | --------------------- |
| core DDL           | `backend/conventions.md`, `backend/registry.md`                                                   | 同左                  |
| core API (OIDC OP) | `backend/patterns.md`, `backend/conventions.md`, `backend/registry.md`, `authorization/matrix.md` | 同左                  |
| examples backend   | `backend/patterns.md`, `backend/conventions.md`                                                   | 同左                  |
| examples frontend  | `frontend/patterns.md`, `frontend/conventions.md`                                                 | 同左                  |
| e2e                | `testing/e2e.md`, `frontend/conventions.md`                                                       | 同左                  |
| 全て共通           | `app/architecture.md`                                                                             | `app/architecture.md` |

### 設計仕様の埋め込み

context に書いてあるパターン・規約は**パス参照**。
タスク固有の仕様 (DDL, API JSON, エラーコード, OIDC scope 等) だけを埋め込む。

### その他

- **個人 PC の絶対パス禁止**: リポジトリルートからの相対パスのみ
- **不明点は推測で埋めない**: 矛盾・欠落を見つけたらユーザーに質問
- **仮実装禁止**: TODO 残し、暫定値の埋め込みで先に進まない

## TDD (全種別で必須)

### バックエンド (Go / Kotlin)

- テスト先行 → 失敗確認 → 実装 → 通過
- カバレッジ目安: Domain 100%, Usecase 95%, Presentation 90% (C0 全体 90% 以上)

### フロントエンド (React / Next.js)

- 純粋関数は **テスト先行。カバレッジ 100%**
- `data-testid` を全 MUST 要素に付与

### E2E

- **シナリオを先に全て書く** (この時点で全て失敗)
- シナリオなしでの完了は認めない

## Codex レビュー (作業ステップに組み込み)

プロンプト全体で 1 回ではなく、**作業単位ごと**にレビューを受ける。
Codex レビューは末尾の補足ではなく、各ステップ手順の一部 (番号付き) として記述する。

```
ステップ N:
  1. テスト書く (TDD)
  2. 実装
  3. lint & test
  4. ★ Codex レビュー実行
  5. 指摘対応 → 次のステップへ
```

Codex には context/ ファイルを渡してレビューさせる:

```bash
codex exec --full-auto -c model_reasoning_effort="medium" \
  "まず以下の context ファイルを読み取れ:
   - ${CONTEXT_DIR}/app/architecture.md
   - ${CONTEXT_DIR}/{該当ファイル}
   その上で git diff をレビューせよ。
   Check: 1) TDD compliance 2) {タスク固有の観点}
   Report issues as CRITICAL/HIGH/MEDIUM/LOW in Japanese."
```
