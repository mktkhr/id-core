---
name: backend-code-review
description: >-
  バックエンドコードを規約・クリーンアーキテクチャ観点でレビューする。"backend-code-review",
  "バックエンドレビュー", "Go コードレビュー" 等で発動。
targets:
  - "*"
---

# Backend Code Review

バックエンド (`core/`, `examples/*/backend/`) の差分を、規約・ベストプラクティスに沿ってレビューする。

## 手順

1. `git status`, `git diff` で現在の差分を確認
2. ブランチ指定があれば `git diff {branch}` で比較
3. 以下の観点で重大度別にレポート

## レビュー観点

### セキュリティ (CRITICAL)

- ハードコードされた認証情報・APIキー・JWT 署名鍵
- SQL インジェクション (生 SQL 直接連結)
- 入力検証の欠如
- OIDC OP として: state / nonce / PKCE 検証漏れ、redirect_uri ホワイトリスト未検証、認可コードの再利用許容
- リダイレクト URL のバリデーション漏れ (open redirect)
- IDトークン/アクセストークンのログ出力

### Go 安全性 (CRITICAL)

- 全角括弧の混入 (Unicode 確認)
- ミューテーション (Domain 層の不変性違反)
- `errors.Is` / `errors.As` の代わりに `==` でエラー比較
- context のリーク (キャンセル伝播の欠如)

### クリーンアーキテクチャ (HIGH)

- Domain 層が外側 (Infrastructure / Presentation) に依存
- Repository **実装**への直接依存 (インターフェース経由でない)
- UseCase が Repository インターフェースを経由せず実装を直接使用
- 機能パッケージ間の直接依存 (`features/A` が `features/B/infrastructure/` を import)
- `shared/` 等の機能間共有パッケージの新設

### コード品質 (HIGH)

- 50 行以上の関数
- 800 行以上のファイル
- 4 レベル以上のネスト
- エラーハンドリングの欠如・`_` でのエラー破棄
- `fmt.Println` / `log.Println` の混入 (構造化 logger を使用)
- TODO / FIXME を残したコミット

### ログ規約 (HIGH)

- 同一エラーの複数層でのログ出力 (重複ログ)
- Domain 層でのログ出力
- 機密情報 (token, password, IDtoken) のログ出力

### ベストプラクティス (MEDIUM)

- マジックナンバー
- `internal/generated/` 配下の手動編集
- API レスポンスで配列に `null` を返却 (`[]` を返す)
- `time.Now()` をエンティティ生成内で直接呼ぶ (テスト困難化)

### 認可 (CRITICAL)

- `docs/context/authorization/matrix.md` (正本) と差分のある実装
- ▲ (条件付き) ロールで所有権チェックの欠如

### Kotlin 固有 (該当時)

- `!!` (non-null assertion) の濫用
- DTO に `data class` でなく可変な class を使用
- `@Transactional` の境界が不適切

## レポート生成

各問題について以下を含むレポートを生成:

- 重要度: CRITICAL, HIGH, MEDIUM, LOW
- ファイル位置と行番号
- 問題の説明
- 修正案

CRITICAL または HIGH が見つかった場合はコミットをブロック。

## 禁止

- git config やリポジトリ設定の変更を提案しない
- コミット・プッシュを実行しない
- セキュリティ脆弱性を見逃したまま承認しない
