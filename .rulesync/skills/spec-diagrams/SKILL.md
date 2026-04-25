---
name: spec-diagrams
description: >-
  設計書に mermaid フローチャート・シーケンス図を生成する。"フローチャート", "シーケンス図", "図を追加", "spec-diagrams", "mermaid" 等で発動。
targets:
  - "*"
---

# mermaid 図生成スキル

設計書のドメイン操作・API 設計からフローチャートとシーケンス図を生成する。

## 前提として読み込む context

**必須** (必ず読む):
- 対象の `docs/specs/{N}/index.md` — 図の元情報

**条件付き** (該当する場合のみ読む):
- シーケンス図でアーキテクチャ規約に従いたい (参加者・トランザクション境界等): `docs/context/backend/patterns.md`
- 認可フローを図に含める: `docs/context/authorization/matrix.md`

**読み込まない**: 図と関係ない context は読まない。

詳細は `docs/context/README.md` の対応表を参照。

## 入力

- `$ARGUMENTS` から対象の設計書パスまたは操作名 (Op1 等) を特定する
- 引数なし: 設計書内の全ドメイン操作に対して生成する

## フローチャート生成ルール

**ユーザー操作起点**で記述する。バックエンド内部処理はシーケンス図に任せる。

```
必須要素:
- 開始: ユーザーのアクション (ボタンクリック等)
- 権限チェック: 認可による表示制御分岐
- フロントバリデーション: 入力チェック → エラー → 再入力ループ
- API 呼び出し: エンドポイントを明記
- レスポンス分岐: 200/201 → 成功、409 → 重複エラー、422 → ドメインエラー、401 → 再ログイン
- 終了: 成功メッセージまたは UI 更新
```

mermaid 構文の注意:

- ラベル内の括弧はダブルクォートで囲む: `-->|"テキスト(補足)"|`
- 日本語テキストはそのまま使用可能

## シーケンス図生成ルール

**バックエンド側の処理を詳細に**記述する。

```
必須参加者 (id-core 本体):
- actor User as ユーザー
- participant FE as Frontend
- participant Core as id-core
- participant Authz as 認可ポリシー
- participant DB as PostgreSQL
- (必要に応じて) participant IdP as 上流 IdP, participant Cache as セッションストア

OIDC OP フローでは:
- participant RP as プロダクト (go-react / kotlin-nextjs)

必須処理:
1. 認可: 全 API で認可ポリシーチェックを記述
2. 所有権チェック: ▲ ロールの場合、認可後に DB 参照で所有権確認
3. トランザクション: BEGIN → 処理 → COMMIT/ROLLBACK を明記
4. エラー分岐: エラーコード一覧の全コードに対応する alt 分岐を記述
   - 401: 認証失敗、トークン無効
   - 403: 認可失敗、所有権なし
   - 404: リソース不在
   - 409: UNIQUE violation、ロック競合、FK 参照削除
   - 422: ドメインバリデーション
5. OIDC 固有: token 検証・nonce/state 検証・PKCE 検証は明示記述
```

## 生成後の検証

生成した図が以下と整合しているか確認する:

- ドメイン操作定義 (Op1 〜 OpN)
- エラーコード一覧
- 認可マトリクス
- API リクエスト仕様
- OIDC OP のフロー (Authorization Code + PKCE 等の標準フロー準拠)
