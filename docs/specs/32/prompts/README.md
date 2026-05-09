# M1.1 実装プロンプト一覧 (Issue #32)

設計書 `docs/specs/32/index.md` から `/spec-prompts` で生成した実装プロンプト群。
実装ツール: **Codex CLI** (gpt-5 系) を想定。

## 依存関係

```
P1_01 (config + keystore + devkeygen)
  ├── P2_01 (Discovery)         並列 OK
  └── P3_01 (JWKS + notimpl)    並列 OK
        └── P4_01 (main 統合 + server route + 統合テスト + context 更新)
```

## ファイル一覧

| ファイル                                  | フェーズ | 並列        | 依存先       | 概要                                                                                                                                          | 行数 |
| ----------------------------------------- | :------: | ----------- | ------------ | --------------------------------------------------------------------------------------------------------------------------------------------- | :--: |
| `P1_01_core_config_keystore_devkeygen.md` |    1     | -           | M0.3 まで    | config 拡張 (`CORE_OIDC_*` + `CORE_ENV` strict) + keystore (KeySet I/F + staticKeySet + kid + 異常系) + devkeygen CLI + Makefile + .gitignore | 317  |
| `P2_01_core_oidc_discovery.md`            |    2     | P3 と並列可 | P1           | Discovery handler + メタデータ + 決定的シリアライズ + ETag + ContractTest 5 ケース                                                            | 313  |
| `P3_01_core_oidc_jwks_notimpl.md`         |    2     | P2 と並列可 | P1           | JWKS handler + notimpl 503 stub + jwx 採用 + golden / 決定論性テスト + private 成分非出力                                                     | 364  |
| `P4_01_core_main_integration_context.md`  |    3     | -           | P1 + P2 + P3 | apperror 拡張 + server.go route 統合 + main.go 起動シーケンス統合 + 統合テスト + context 更新 + README 加筆                                   | 349  |

## 実装手順 (推奨)

1. **P1 を Issue 化** (`/issue-from-spec 32` または手動 `gh issue create`)、M1.1 マイルストーン + `種別:実装` + `対象:基盤` ラベル
2. P1 の Issue を Codex CLI に渡して実装 → PR → `/pr-codex-review` → マージ
3. P2 と P3 を並列で Issue 化、Codex CLI で実装 (別 PR で同時進行可能)
4. P4 を Issue 化 (P1+P2+P3 マージ完了が前提)、Codex CLI で実装 → PR → マージ
5. 親 Issue #32 に「M1.1 完了」コメント → ユーザーが手動 Close

## 各プロンプトの絶対ルール (共通)

全プロンプトに以下を含めている:

- コードベース探索禁止 (Grep / Glob / Read 探索 / Explore agent 禁止)
- Codex レビュー必須 (各ステップごと、ゲート CRITICAL=0 / HIGH=0 / MEDIUM<3)
- 完了条件のタスク化必須 (作業開始前に TaskCreate で全項目登録)
- `apperror.WriteJSON` を使う / 使わないの判別 (P4 のみ apperror 拡張、P3 notimpl は使わない)
- 設計書 / 要求文書からの該当部分埋め込み (自己完結型、`.rulesync/rules/path-resolution.md` 準拠)

## 設計書からのトレーサビリティ

各プロンプトは設計書の以下セクションと対応:

| プロンプト | 設計書セクション                                                                            | 要求 F-ID                                   |
| ---------- | ------------------------------------------------------------------------------------------- | ------------------------------------------- |
| P1_01      | 既存実装からの統合点 / 新規追加されるパッケージ / 環境変数 / 論点 #1 #5 #9 #10 #13 #14 #16  | F-7, F-8, F-9, F-11, F-13, F-14, F-15, F-18 |
| P2_01      | API 設計 / Discovery レスポンスヘッダ / 論点 #4 #6 #7 #12 / フロー図 (Discovery)            | F-1, F-2, F-3, F-4, F-15, F-16, F-17, F-21  |
| P3_01      | API 設計 / JWKS レスポンスヘッダ / 503 stub / 論点 #4 #7 #8 #10 / フロー図 (JWKS, 503 stub) | F-5, F-6, F-15, F-16, F-21, F-23            |
| P4_01      | 既存実装からの統合点 / 既存資料からの差分 / フロー図 (起動シーケンス)                       | F-8, F-9, F-15, F-18, F-19, F-20            |
