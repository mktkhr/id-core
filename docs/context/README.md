# context/ — 実装コンテキスト集約ディレクトリ

## 目的

設計フェーズで得た知識を凝縮し、実装フェーズでの再探索を不要にする。

```
設計  →  context/ に書き出す (生産者)
実装  →  context/ を読む (消費者) + 不足を追記 (生産者)
レビュー  →  context/ との乖離を指摘
```

## ディレクトリ構成

```
context/
├── README.md                  # このファイル
├── app/
│   └── architecture.md        # 技術スタック・モノレポ構成・認証プロトコル方針
├── authorization/
│   └── matrix.md              # 認可マトリクス正本 (唯一の実体)
├── backend/
│   ├── conventions.md         # id-core (Go) の DB/API/エラー/認可規約
│   ├── patterns.md            # 実装パターン
│   └── registry.md            # テーブル一覧・API一覧・エラーコード一覧
├── frontend/
│   ├── conventions.md         # React / Next.js サンプルでの規約
│   ├── patterns.md            # フォーム・OIDC クライアント実装パターン
│   └── registry.md            # 画面一覧・data-testid 規約
└── testing/
    ├── backend.md             # Go テストパターン・カバレッジ基準
    └── e2e.md                 # Playwright パターン・data-testid 規約
```

## skill → 必読 context 対応表

各 skill が実行前に読み込むべき context ファイルの**正本マッピング**。
**「必読」と「条件付き」を明確に分離**することで、無関係な context を読み込んで token を浪費しないようにする。

| Skill                   | 必読 (常に読む)                                                      | 条件付き (該当時のみ読む)                                                                                                                                             |
| ----------------------- | -------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `plan`                  | `app/architecture.md` (北極星)                                       | 関連 `requirements/{N}/`, `specs/{N}/` (既存あれば)                                                                                                                   |
| `survey`                | —                                                                    | 調査対象に応じて選択                                                                                                                                                  |
| `check-convention`      | —                                                                    | 引数で指定された観点に応じて (skill 内表で詳細指定)                                                                                                                   |
| `doc-review`            | レビュー対象本体                                                     | レビュー対象が含む領域に応じて: backend/_ (DB/API/エラー) / frontend/_ (画面) / testing/\* (テスト) / authorization/matrix.md (認可) / app/architecture.md (全体整合) |
| `adr-create`            | `docs/templates/adr/template.md`                                     | `app/architecture.md` (影響範囲分析時) / 既存 `adr/*.md` (関連既存決定の有無)                                                                                         |
| `commit`                | —                                                                    | (不要)                                                                                                                                                                |
| `requirements-create`   | `docs/templates/requirements/template.md`                            | 認可記述あり: `authorization/matrix.md` / 初回・スコープ把握不足: `app/architecture.md`                                                                               |
| `requirements-resolve`  | 対象 `requirements/{N}/index.md`                                     | 認可論点: `authorization/matrix.md`                                                                                                                                   |
| `requirements-review`   | 対象 `requirements/{N}/index.md`                                     | 認可記述あり: `authorization/matrix.md`                                                                                                                               |
| `requirements-validate` | 対象 `requirements/{N}/index.md`                                     | 認可記述あり: `authorization/matrix.md` (突合済かの確認)                                                                                                              |
| `requirements-track`    | 対象 `requirements/{N}/index.md`                                     | (なし)                                                                                                                                                                |
| `requirements-full`     | (フェーズ毎に下位 skill の必読 context のみ)                         | (フェーズ毎に下位 skill の条件付き context)                                                                                                                           |
| `spec-create`           | `docs/templates/specs/template.md`, 該当 `requirements/{N}/index.md` | 認可記述あり: `authorization/matrix.md` / 初回・全体把握不足: `app/architecture.md`                                                                                   |
| `spec-resolve`          | 対象 `specs/{N}/index.md`                                            | 論点が認可: `authorization/matrix.md` / 論点が DB/API: `backend/{registry,conventions}.md` / 論点が画面: `frontend/{registry,conventions}.md`                         |
| `spec-tests`            | 対象 `specs/{N}/index.md`                                            | バックエンドテスト: `testing/backend.md` / E2E: `testing/e2e.md` + `frontend/registry.md` / 認可テスト: `authorization/matrix.md`                                     |
| `spec-diagrams`         | 対象 `specs/{N}/index.md`                                            | アーキテクチャ規約準拠: `backend/patterns.md` / 認可フロー: `authorization/matrix.md`                                                                                 |
| `spec-track`            | 対象 `specs/{N}/index.md`                                            | (なし)                                                                                                                                                                |
| `spec-full`             | (フェーズ毎に下位 skill の必読 context のみ)                         | (フェーズ毎に下位 skill の条件付き context)                                                                                                                           |

### 読み取りの原則

- **「必読」のみ自動読み込み**: skill 起動時に確実に読むのは「必読」列のファイルのみ
- **「条件付き」は条件成立時のみ読む**: トリガー条件 (例: 認可記述あり) を skill 自身が判定してから読む
- **無関係な context は絶対に読まない**: 例えば `spec-tests` でバックエンドテストだけ生成する場合、frontend/\* は読まない
- **架空のファイル探索禁止**: ディレクトリツリーを `ls` で漁って関係ありそうなファイルを開く、といった行為は token の浪費なので行わない
- **架空の "全部読み" 禁止**: 「念のため全 context を読む」「広く読んでおく」は禁止
- **同一セッション内では再読込不要**: 一度読んだ context は会話履歴に残るので、同じセッション内で再 Read しない
- **更新時の責務**: skill が context を読んで内容が古いと判明した場合、その skill 内で更新する (生産者・消費者モデル)

## 鮮度の維持

各ファイルの冒頭に最終更新日を記載する:

```markdown
> 最終更新: YYYY-MM-DD
```

実装/設計セッションは、この日付と現在の状態を照合し、古い場合は更新する。

## 何を書くか

- **事実のみ** (規約・パターン・一覧・制約)
- 議論経緯は書かない (それは `specs/` や `adr/` の役割)
- コードは最小限の例示のみ (全文コピーしない)
- 陳腐化しやすい数値 (マイグレーション番号等) は registry に集約
