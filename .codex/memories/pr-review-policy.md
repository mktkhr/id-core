# PR レビュー / メタデータ / バージョン変更ポリシー

Pull Request 作成時の必須事項。**省略・後回し禁止 (Auto モードでも適用)**。

## 1. Codex レビュー必須

全 PR は **Codex レビューゲート** を通過してからマージする。

### 手順

1. ブランチ作成 → 実装 → push
2. `gh pr create` で PR 起票
3. **直後に `/pr-codex-review {PR 番号}`** で Codex に diff + description を投げる
4. ゲート判定:
   - **CRITICAL = 0**
   - **HIGH = 0**
   - **MEDIUM < 3**
5. ゲート未達 → 修正 → push → 再レビュー (ループ)
6. ゲート通過後にマージ (`--merge` がデフォルト、`--squash` は履歴をまとめる場合のみ)

### レビュー観点 (Codex に必ず確認させる)

- PR description にアーカイブ参照や固有事情が混入していないか (本ルール §4 参照)
- 誤メンションを引き起こす裸の `@` 表記が残っていないか (本ルール §5 参照)
- ランタイム / 依存 / Action のバージョンが古くないか or 独断変更になっていないか (本ルール §3 参照)
- 差分が PR スコープに収まっているか (関係ないファイルが混ざっていないか)
- 重複コード / 命名違反 / 規約違反
- スコープと完了条件の整合
- PR 本文と差分の整合

「PR を作って即マージ、Codex レビュー省略」は禁止。Auto モードでも Codex レビューは省略しない。

## 2. メタデータ必須 (assignee + labels)

PR 起票時は `--assignee` と `--label` を必ず指定する。

```bash
gh pr create \
  --assignee @me \
  --label "種別:..." \
  --label "対象:..." \
  --base main --head <branch> \
  --title "..." --body "..."
```

### 必須

- **assignee**: 作業者 (`--assignee @me` で十分。CI が実行できるアカウントを指定)
- **labels**:
  - 種別ラベル (必須、いずれか 1 つ): `種別:機能追加` / `種別:バグ` / `種別:改善` / `種別:設計` / `種別:実装` / `種別:調査`
  - 対象ラベル (推奨): `対象:基盤` / `対象:サーバー` / `対象:画面`
  - 優先度ラベル (任意): `優先:高` / `優先:中` / `優先:低`

ラベル定義は `.rulesync/rules/issue-traceability.md` を参照。

### 後追い修正

メタデータ漏れを後で気づいた場合は GitHub API 経由で修正:

```bash
gh api -X POST "repos/{owner}/{repo}/issues/{番号}/assignees" -f "assignees[]={username}"
gh api -X POST "repos/{owner}/{repo}/issues/{番号}/labels" -f "labels[]={label}"
```

`gh pr edit` は GraphQL の Projects classic 廃止由来でエラーが出る場合があるため `gh api` 直叩きが確実。

## 3. バージョン変更は事前承認必須

`go.mod` の `go` ディレクティブ、`package.json` の `engines`、`.nvmrc`、Dockerfile のベースイメージ、GitHub Action の `uses` の major/minor、setup-go / setup-node の `go-version` / `node-version` 等で、**ランタイム・依存・ツールのバージョンを変更する場合は事前にユーザーへ提案して承認を得る**。

### 禁止

- **「CI を通すためにランタイム / 依存のバージョンを下げる」は典型的 NG**
  - CI 側 (Action / ツール) を修正して対応するか、Action のバージョンを上げる方向で解決する
  - go.mod / package.json は要求の正本。CI の都合で書き換えない
- 上げる場合 (Active LTS 採用等) も同様に承認を得る

### 例外

自動生成物の再生成 (`make rulesync` 等) で生まれる差分のみ承認不要。

### 失敗時のリカバリ

ゴミコミットを作ってしまったら **revert ではなく `git reset --hard` + `git push --force-with-lease`** で履歴から完全に消す。revert で「やった事実」を残さない。

## 4. PR description にアーカイブ・固有事情を書かない

公開リポジトリ前提なので、リポジトリ外から見える文章 (PR description / Issue 本文 / commit message / docs/ 配下) で、`アーカイブ/` 配下の参考リポジトリや特定の固有プロジェクト名・社内事情に言及しない。

- 「アーカイブからの差分」「ナンダカンダから持ってきた」等の記述禁止
- 必要なら抽象表現 (「参考にした内容」程度) で済ませるか、そもそも書かない
- 例外的にアーカイブ参照を残してよいのは、ユーザーが明示的に求めた場合のみ

## 5. 誤メンションを発生させない

PR / Issue / コメント本文で `@<word>` 形式の文字列を**裸で書かない**。GitHub の自動メンションで無関係なユーザーに通知が飛ぶ。

### 典型的な誤メンション源

- GitHub Action の `uses` バージョン参照: `@v1` `@v2` `@master` `@latest`
- Docker タグ: `image@sha256:...`
- git ref: `@HEAD` `@main`

### 対策

1. **インラインコード (` `` `) で囲む**: 例 `` `securego/gosec@v2.26.1` ``
2. それでも一部表現は誤メンション化されることがあるため、不安なら **`@` を含まない言い換え**にする
   - 「`@master` 指定」→「master ブランチ参照」
   - 「`@v2` 固定」→「v2 系を固定」
3. コードブロック (```yaml ... ```) 内なら通常メンション化されない (yaml サンプルは安全)

### 確認手順

PR description / Issue 本文を書いた直後に grep で確認:

```bash
echo "$BODY" | grep -nE '(^|[^`])@[A-Za-z0-9_-]+'
# 出力があるバッククォート外の @ 表記をすべてエスケープ or 言い換え
```

## 6. PR タイトル / コミットメッセージ規約

- コミット規約は `.rulesync/skills/commit/SKILL.md` 参照 (`{type}:{emoji} {対象}`)
- PR タイトルもコミット規約に揃える (`feat:✨` / `fix:🐛` / `docs:📝` / `chore:🔧` / `refactor:♻️` / `test:✅`)
- 関連 Issue がある場合は PR 本文に `Closes #N` を含める
