---
name: issue-from-spec
description: >-
  設計書から実装タスク Issue を分割起票する。"設計書から issue 化", "実装タスクを起票",
  "issue-from-spec", "設計書からチケット分割" 等で発動。
targets:
  - "*"
---

# 設計書 → Issue 分割起票スキル

設計書の実装プロンプト構成をもとに、実装タスク Issue を分割起票する。
親 (要求 Issue) -子 (実装 Issue) の関係、および依存順序を明示する。

このリポジトリは**モノレポ**なので、リンクは `#番号` で完結する (フルパス不要)。

## ワークフロー

```
設計書読み取り → タスク分割案作成 → ドライラン提示 (ツリー表示) → ユーザー承認
→ 実装 Issue を一括起票 → 親 Issue 本文に task list 追記
```

## 起票先の判定

実装プロンプトのファイル名 / 設計書の対象領域から、適用範囲を判定する:

| プロンプト名のキーワード         | 適用範囲 (本文に明記)                       |
| -------------------------------- | ------------------------------------------- |
| `core`, `op`, `migration`        | `core/` (OIDC OP 本体)                      |
| `go-react-backend`, `go-backend` | `examples/go-react/backend/`                |
| `go-react-frontend`, `react`     | `examples/go-react/frontend/`               |
| `kotlin`, `spring`               | `examples/kotlin-nextjs/backend/`           |
| `nextjs`, `next`                 | `examples/kotlin-nextjs/frontend/`          |
| `e2e`                            | E2E (`examples/*/frontend/` 配下に置く想定) |

> モノレポなので別リポジトリへの起票はない。Issue は本リポジトリ内で完結する。

## 命名規則

### 種別プレフィックス (必須)

全 Issue の件名に種別プレフィックスを付与する。一覧での識別性を確保する。

| 種別 | プレフィックス |
| ---- | -------------- |
| 設計 | `【設計】`     |
| 実装 | `【実装】`     |

### 件名フォーマット

```
{種別プレフィックス}{機能名}: {スコープ}
```

例:

- `【実装】OIDC 認可コードフロー: トークンエンドポイント`
- `【実装】OIDC 認可コードフロー: ログイン画面 (go-react)`
- `【実装】電話番号認証: SMS 送信基盤`
- `【実装】Federation 集約: E2E テスト`

## 親子関係の表現

### 方法 1: 親 Issue の task list (必須・自動)

親 (要求) Issue の本文に `- [ ] #N — タイトル` 形式で実装 Issue を並べる。
GitHub は同一リポジトリ内の Issue を自動認識し、close 時にチェックを付ける。

### 方法 2: GitHub の sub-issue 機能 (任意・UI 手動)

GitHub の Issue 階層機能 (sub-issue) が利用可能な場合、UI で親子を設定するとツリー表示される。
スキルは方法 1 を自動実行し、方法 2 は案内のみ行う。

## 手順

### 1. 設計書の読み取り

`$ARGUMENTS` から設計書パスを特定する。設計書から以下を抽出:

- 実装プロンプト構成 (`docs/specs/{N}/prompts/` のフェーズ・順序依存関係)
- 機能名 (件名プレフィックスに使用)
- 対象領域 (`core/` / `examples/...`)
- 元の要求 Issue 番号 (設計書冒頭または `docs/specs/{番号}/` のディレクトリ名)

### 2. タスク分割案の作成

実装プロンプトの構成に基づいて Issue を定義する:

- 実装プロンプトのフェーズ 1 つ = 1 つ以上の Issue
- 件名は上記「命名規則」に従う
- 各 Issue 本文に以下を**必ず含める**:
  - 目的 (設計書から抽出)
  - 実装プロンプトへの参照 (`docs/specs/{N}/prompts/P{x}_{y}_*.md`)
  - 完了条件 (チェックボックスで列挙)
  - **元 Issue リンク (`## 元 Issue` セクション、絶対必須)**
  - **前提 Issue** (依存がある場合)
- ラベル: `種別:実装` (または `種別:設計`) + 対象ラベル

### 3. ドライラン提示 (必須 — スキップ禁止)

起票前に**必ずツリー形式**で一覧を表示してユーザーの承認を得る。

> **ガード**: ドラフトをユーザーに提示せずに起票してはならない。

```
## Issue 分割プレビュー

親: #42 (OIDC 認可コードフロー)

子 Issue (起票予定):

  ├─ [P1] 【実装】OIDC 認可コードフロー: DB 基盤            (core/)
  │         ラベル: 種別:実装, 対象:基盤
  │
  ├─ [P2] 【実装】OIDC 認可コードフロー: トークンエンドポイント (core/)
  │         前提: P1
  │         ラベル: 種別:実装, 対象:サーバー
  │
  ├─ [P3] 【実装】OIDC 認可コードフロー: ログイン画面       (examples/go-react/frontend/)
  │         前提: P2
  │         ラベル: 種別:実装, 対象:画面
  │
  └─ [P4] 【実装】OIDC 認可コードフロー: E2E              (examples/go-react/frontend/)
            前提: P3
            ラベル: 種別:実装, 対象:画面

依存関係:
  P1 → P2 → P3 → P4  (順次実行)

この内容で起票してよいですか? (Y/N)
```

### 4. マイルストーン継承 (必須)

詳細は `.rulesync/rules/issue-traceability.md` 参照。

**親要求 Issue のマイルストーンを取得し、全ての子 Issue を同じマイルストーンに紐付ける**:

```bash
# 親要求 Issue のマイルストーン取得
PARENT_MILESTONE=$(gh issue view {親 Issue 番号} --json milestone --jq '.milestone.title // empty')

# 親にマイルストーンが紐付いていない場合は停止 (スキルが勝手に推測しない)
if [ -z "$PARENT_MILESTONE" ]; then
  echo "親要求 Issue にマイルストーンが紐付いていません。先に親側を更新してください。"
  exit 1
fi
```

ユーザーに確認: 「親要求 Issue (#{N}) のマイルストーン \`$PARENT_MILESTONE\` に全子 Issue を紐付けます。よろしいですか?」

### 5. Issue 一括起票

承認後、各 Issue を `gh issue create` で起票する (**マイルストーン紐付け必須**)。
**親 Issue への `## 元 Issue` リンクは全 Issue に必須**。

```bash
gh issue create \
  --title "【実装】OIDC 認可コードフロー: DB 基盤" \
  --body "$(cat <<'EOF'
## 目的
{設計書から抽出した目的}

## 実装プロンプト
docs/specs/0001/prompts/P1_01_core_db.md

## 適用範囲
core/

## 完了条件
- [ ] マイグレーション作成
- [ ] sqlc / クエリ定義
- [ ] ユニットテスト

## 元 Issue
#42
EOF
)" \
  --milestone "$PARENT_MILESTONE" \
  --label "種別:実装" \
  --label "対象:基盤"
```

### 6. 親 Issue への task list 追記 (必須)

起票完了後、親 Issue の本文末尾にタスクリストを追記する:

```bash
# 既存本文取得
PARENT_BODY=$(gh issue view 42 --json body --jq .body)

gh issue edit 42 --body "${PARENT_BODY}

## 実装タスク

- [ ] #{N1} — 【実装】OIDC 認可コードフロー: DB 基盤
- [ ] #{N2} — 【実装】OIDC 認可コードフロー: トークンエンドポイント
- [ ] #{N3} — 【実装】OIDC 認可コードフロー: ログイン画面
- [ ] #{N4} — 【実装】OIDC 認可コードフロー: E2E
"
```

GitHub は同一リポジトリ内の `#番号` を自動でリンク化し、close 時にチェックを付ける。

### 7. (任意) sub-issue 設定の案内

GitHub の sub-issue 機能を使う場合、UI からの設定をユーザーに案内する。
CLI からの設定は GraphQL が必要なため、本スキルでは案内のみとする。

## 元 Issue リンク必須ルール (絶対)

実装系 Issue (`【実装】` / `【設計】`) の本文に、以下の形式で親 Issue へのリンクを**必ず含める**:

```markdown
## 元 Issue

#{親 Issue 番号}
```

詳細は `.rulesync/rules/issue-traceability.md` を参照。

## 自動 Close

PR 本文に `Closes #N` を書けば、その PR がマージされた際に対応 Issue が自動 close される。
親要求 Issue は実装完了後にユーザーが手動 Close する。

## バリデーション

起票前に以下を検証:

1. **必須フィールド**: 件名・本文が空でないこと
2. **種別プレフィックス**: 件名が `【設計】` または `【実装】` で始まること
3. **元 Issue リンク**: 全子 Issue 本文に `## 元 Issue` セクションが存在すること
4. **マイルストーン**: 親要求 Issue にマイルストーンが紐付いており、全子 Issue にも継承されていること
5. **依存の循環**: 前提 Issue が循環していないこと
6. **適用範囲の存在**: 実装プロンプト名から判定したパス (`core/`, `examples/...`) が実在すること

違反時は起票を中断し、違反項目を明示する。

## 設計 Issue のスキップ条件

以下の場合は `【設計】` Issue を起票せず、`【実装】` のみ起票してよい:

- スコープが小さい (1 プロンプト分で完結する)
- 設計書側で既に詳細が確定している
- ユーザーが明示的にスキップを指定

## 注意

- Milestone は未設定で起票する (後から計画時に設定)
- 依存関係は本文に「前提: #N」と記載する
- task list は GitHub が自動でチェックボックス化するため、素の `- [ ] #N` 形式で書く
