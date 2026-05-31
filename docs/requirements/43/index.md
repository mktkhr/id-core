# 要求 #43: 認可エンドポイント (/authorize) を core/ に導入し認可コードを発行する

- 関連 Issue: [#43](https://github.com/mktkhr/id-core/issues/43)
- マイルストーン: [M1.2: 認可エンドポイント (/authorize) - 認可コード発行](https://github.com/mktkhr/id-core/milestone/5)
- 状態: 設計着手OK
- 起票日: 2026-05-31
- 最終更新: 2026-05-31

## 背景・目的

M1.1 (#32) で公開したメタデータ上の `authorization_endpoint` を実体化し、id-core を OIDC OP として authorization_code フローの起点にする。RP が `/authorize` に認可リクエストを送り、認可コードを受け取って `redirect_uri` に戻れる状態を作る。

このマイルストーンが完了しない限り、以下が実現できない:

- M1.3: RP が認可コードを `/token` で交換できない (交換すべきコードがそもそも発行されない)
- M1.5: RP の OIDC ライブラリ (例: go-oidc) が authorization_code フローを開始できない

M1.1 で Discovery が広告し、現在 HTTP `503` stub (M1.1 F-23) を返している `authorization_endpoint` を、本マイルストーンで実体に置き換える。

スコープは **`/authorize` の認可リクエスト受付・検証 + 認可コード発行 + リダイレクト**であり、トークン発行・`code_verifier` 照合・ユーザー DB・Consent 画面・クライアント登録 DB は対象外 (後続マイルストーンで実装)。

起動前提は M1.1 と同じく **K8s** (HPA で複数 Pod 想定)。Pod をまたいで認可コードフロー (`/authorize` 発行 → `/token` 交換) が成立する必要があるため、認可コードは Pod 間で共有されるストアに保存することが必須 (インメモリのみの実装は不可)。reverse proxy / Ingress 経由の TLS 終端を前提とする。

## ユーザーシナリオ

### 開発者 (RP 側、id-core を OP として接続したい)

1. RP の OIDC クライアントライブラリが Discovery (M1.1) から `authorization_endpoint` を取得
2. ユーザーをブラウザで `GET ${authorization_endpoint}?response_type=code&client_id=...&redirect_uri=...&scope=openid&state=...&code_challenge=...&code_challenge_method=S256` にリダイレクト
3. id-core が固定ユーザーを認証済みとみなし、認可コードを発行
4. id-core が `redirect_uri?code=<認可コード>&state=<受信した state>` にリダイレクトして RP へ戻す
5. RP は受け取った `code` を M1.3 (`/token`) で交換する (M1.2 範囲では交換先は未実装)
6. PKCE / パラメータ不正時は OAuth2 / OIDC Core 準拠のエラーで応答 (リダイレクト可否は後述 F-12)

### 開発者 (id-core 自身を開発)

1. M1.1 と同じ手順で dev 鍵・DB・issuer を準備して id-core を起動
2. 設定で固定クライアント (`client_id` / `redirect_uri` / `scope`) と固定の認証済みユーザー (`sub`) を定義
3. ブラウザまたは `curl -i` で `/authorize` に認可リクエストを送り、`302` リダイレクトの `Location` に `code` と `state` が乗ることを確認
4. PKCE 未指定 / `response_type` 不正 / `redirect_uri` 不一致など異常系のエラー応答を確認

## 用語

- **要求文書**: 本ファイル (`docs/requirements/43/index.md`)
- **OP (OpenID Provider)**: OIDC で ID Token を発行する側 (id-core はこちら)
- **RP (Relying Party)**: OP に認証を委譲する側 (M1.5 で go-react サンプルが該当)
- **認可エンドポイント (`/authorize`)**: RP からの認可リクエストを受け、エンドユーザー認証後に認可コードを発行する OIDC OP のエンドポイント (OpenID Connect Core 1.0 / RFC 6749)
- **認可コード (authorization code)**: `/authorize` が発行し、後続の `/token` で ID Token / Access Token と交換される短命・単回使用のコード
- **PKCE (Proof Key for Code Exchange)**: 認可コード横取り攻撃対策 (RFC 7636)。`/authorize` で `code_challenge` を受け取り、`/token` で `code_verifier` と照合する
- **固定クライアント**: M2.1 のクライアント登録 DB 導入前の暫定として、設定ファイル / env で静的に定義するクライアント
- **固定ユーザー**: M3.x のユーザー DB / ローカル認証導入前の暫定として、設定で定義する認証済みユーザー (`sub`)

## 編集ルール (本ファイル限定)

- 本ファイル内では `@` 記号を裸で書かない (`@v1` / `@master` / `@user` 等)。GitHub の自動メンション混入を防ぐため、必ずバッククォートで囲むか別表現に言い換える
- 既存の規約は [.rulesync/rules/pr-review-policy.md](../../../.rulesync/rules/pr-review-policy.md) §5 を参照

## 機能要件

| ID   | 内容                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| F-1  | `/authorize` エンドポイントを新設し、M1.1 で広告済みの `authorization_endpoint` を実体化する。M1.1 F-23 の `503` stub を本エンドポイント実装で置き換える。`/authorize` は HTTP **`GET` と `POST` の両方**をサポートする (OpenID Connect Core 1.0 §3.1.2.1 で両対応が MUST。`POST` 時はパラメータを `application/x-www-form-urlencoded` body で受ける)                                                                                                                                                                                |
| F-2  | `response_type` は **`code` のみ**サポートする。それ以外の値は `unsupported_response_type` エラー                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| F-3  | `scope` は必須。`scope` 欠落・形式不正は `invalid_request`、`scope` はあるが **`openid` を含まない**場合は `invalid_scope`。`openid` 以外の未対応 scope は**無視する** (OpenID Connect Core 1.0 §3.1.2.1 の「未理解 scope は ignore」SHOULD に準拠)。M1.1 Discovery の `scopes_supported = ["openid"]` と整合する。詳細は「エラーコード対応」表で一意に定める                                                                                                                                                                        |
| F-4  | `client_id` は必須で**固定クライアント設定に存在する値**であること。欠落 / 未知の `client_id` はリダイレクト先を信頼できないため `invalid_request` として F-12 に従い**直接エラー表示**する (リダイレクトしない)。クライアントは判明しているが authorization_code フローの利用が許可されていない場合は `unauthorized_client` を返す。詳細は「エラーコード対応」表で一意に定める                                                                                                                                                      |
| F-5  | `redirect_uri` は必須で、対象 `client_id` の**登録値と完全一致**すること (部分一致・前方一致は不可)。不一致 / 欠落は F-12 によりリダイレクトせず直接エラー表示                                                                                                                                                                                                                                                                                                                                                                       |
| F-6  | PKCE: `code_challenge` 必須、`code_challenge_method` は **`S256` のみ**許可する。`plain` / 未指定 / 未知の method は `invalid_request` エラー。`/authorize` では `code_challenge` / `code_challenge_method` の存在・形式検証と**認可コードへの束縛**までを行い、`code_verifier` との照合は M1.3 (`/token`) の責務とする                                                                                                                                                                                                              |
| F-7  | `state` は任意。受信した場合は成功・エラーいずれのリダイレクトでも**そのまま返却**する (改変しない)。`state` が要求に含まれない場合は返却しない。CSRF 対策として RP には付与を強く推奨する旨を規約書に記す                                                                                                                                                                                                                                                                                                                           |
| F-8  | `nonce` は任意。受信した場合は**認可コードに束縛して保存**し、M1.3 で ID Token の `nonce` claim に反映できるようにする (M1.2 では束縛・保存までが責務、ID Token 生成は M1.3)                                                                                                                                                                                                                                                                                                                                                         |
| F-9  | 検証に成功した場合、固定ユーザーを認証済みとみなして認可コードを発行し、`redirect_uri` に `code` (および `state` があれば `state`) を付与して **HTTP リダイレクト**で RP へ戻す                                                                                                                                                                                                                                                                                                                                                      |
| F-10 | 発行した認可コードは **`client_id`・`redirect_uri`・PKCE `code_challenge`(+`code_challenge_method`)・(受信時は) `nonce`・認証済みユーザー `sub` に束縛**して保存する (RFC 6749 §4.1.2)。M1.3 (`/token`) はこの束縛を用いて `client_id`・`redirect_uri` の一致確認と `code_verifier` 照合を行う。保存先は **HPA 複数 Pod 環境でも M1.3 で交換可能**なよう Pod 間で共有されるストアとする (インメモリのみの実装は不可)。共有ストアの技術選択 (DB 先行 / Redis 等) と TTL・一意性・単回使用の扱いは検討事項 Q1 で設計フェーズに委譲する |
| F-11 | パラメータ検証時に返すエラーコードは下記「エラーコード対応」表で一意に定める。原則は必須欠落・パラメータ重複・形式不正 = `invalid_request` だが、`response_type` 不正 = `unsupported_response_type`、`openid` 不在 = `invalid_scope` を専用コードとして優先する。F-3 / F-4 の個別記述は本表に従う                                                                                                                                                                                                                                    |
| F-12 | エラー応答は **RFC 6749 / OpenID Connect Core 1.0 準拠**。`redirect_uri` または `client_id` が不正・欠落で**リダイレクト先を信頼できない場合はリダイレクトせず直接エラーを表示**する。それ以外のエラーは `redirect_uri` に `error` / `error_description` (任意で `error_uri`) を付与して返す。エラーの構造化ログ / 表現は M0.2 (#7) で確定したエラー規約に接続する                                                                                                                                                                   |
| F-13 | サポートするエラーコードの最低セット: `invalid_request` / `unauthorized_client` / `unsupported_response_type` / `invalid_scope` / `access_denied` / `server_error` / `temporarily_unavailable` (RFC 6749 §4.1.2.1)                                                                                                                                                                                                                                                                                                                   |
| F-14 | 認証の拒否 / フロー中断時は **`access_denied`** を返す経路を用意する (Consent 画面そのものは M3.3 のため、M1.2 ではエラー経路の存在のみを担保)                                                                                                                                                                                                                                                                                                                                                                                       |
| F-15 | ユーザー認証は固定。設定で定義した**仮の認証済みユーザー (`sub`)** を用いる。ログイン画面・ユーザー DB・パスワード認証は M3.x で導入する                                                                                                                                                                                                                                                                                                                                                                                             |
| F-16 | クライアントは設定で固定定義する (`client_id` / 登録 `redirect_uri` / 許可 `scope` 等)。クライアント登録 DB + 管理 API は M2.1 で導入する                                                                                                                                                                                                                                                                                                                                                                                            |
| F-17 | ロール別認可は行わない。`/authorize` はエンドユーザーの OAuth2 認可フローであり、認可マトリクス (ロール × 機能) の対象外とする。マスター (`docs/context/authorization/matrix.md`) のロール認可は M3.x (ロール導入時) で反映する                                                                                                                                                                                                                                                                                                      |
| F-18 | `/authorize` は M0.2 で確定した middleware チェーン順序 (`request_id → access_log → recover → handler`、#32 Q14) に乗せる。`/authorize` 特有の middleware (rate limiting / CSRF 等) が必要な場合は検討事項として設計フェーズで判断する                                                                                                                                                                                                                                                                                               |
| F-19 | 構造化ログに `request_id` を含める。**認可コード値 (`code`) ・`code_verifier` 等の秘匿情報はログに出力しない** (M0.3 / M1.1 の redact 方針を踏襲)。`code_challenge` は PKCE 仕様上の公開値 (challenge は公開され、`code_verifier` が秘密) のため秘匿対象から除外する。エラー時は `error` コードと検証失敗理由を記録する                                                                                                                                                                                                              |
| F-20 | テストカバレッジの最低ライン: (a) 正常系 (全必須パラメータ + PKCE S256) で `code` + `state` が `redirect_uri` に返る、(b) `response_type` 不正 / `scope` に `openid` なし / PKCE 不正 / 必須欠落の各異常系、(c) `redirect_uri` / `client_id` 不正時にリダイレクトせず直接エラーになる (F-12)、(d) `nonce` がコードに束縛される (M1.3 で参照可能なこと)                                                                                                                                                                               |

## エラーコード対応

パラメータ検証・認可フローで返すエラーコードと、リダイレクト可否を以下に一意に定める (RFC 6749 §4.1.2.1 / OpenID Connect Core 1.0)。`error_description` の文言は M0.2 (#7) のエラー規約に従う。

| 条件                                             | `error`                     | リダイレクト              |
| ------------------------------------------------ | --------------------------- | ------------------------- |
| `client_id` 欠落 / 未知                          | `invalid_request`           | しない (直接エラー、F-12) |
| `redirect_uri` 欠落 / 登録値と不一致             | `invalid_request`           | しない (直接エラー、F-12) |
| `response_type` が `code` 以外                   | `unsupported_response_type` | する                      |
| `scope` 欠落 / 形式不正                          | `invalid_request`           | する                      |
| `scope` に `openid` を含まない                   | `invalid_scope`             | する                      |
| `code_challenge` 欠落                            | `invalid_request`           | する                      |
| `code_challenge_method` が `S256` 以外 / 欠落    | `invalid_request`           | する                      |
| その他必須欠落・同一パラメータ重複・形式不正     | `invalid_request`           | する                      |
| クライアントが authorization_code フロー利用不可 | `unauthorized_client`       | する                      |
| 認証拒否 / フロー中断                            | `access_denied`             | する                      |
| 共有ストア障害等の一時的失敗                     | `temporarily_unavailable`   | する                      |
| 上記以外の内部エラー                             | `server_error`              | する                      |

- `client_id` / `redirect_uri` が信頼できずリダイレクト先を確定できない場合を除き、エラーは `redirect_uri` に `error` / `error_description` (任意で `error_uri`) と `state` (受信時) を付与して返す (F-7 / F-12)。
- リダイレクト不可ケース (上表「しない」) は `redirect_uri` にエラーを載せず、直接エラー応答する (open redirect 防止)。

## 非機能要件

| 区分           | 要件                                                                                                                                                                                                                                                                 |
| -------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| セキュリティ   | PKCE は `S256` 必須 (F-6)。認可コードは短命・単回使用 (TTL / 単回使用の具体値は Q1 で設計確定)。open redirect 防止のため `redirect_uri` は完全一致検証 (F-5)。秘匿情報をログに出さない (F-19)。host header / `X-Forwarded-*` を信頼しない (Ingress 前提、#32 と整合) |
| 可用性         | HPA 複数 Pod 環境でも `/authorize` → `/token` の認可コードフローが成立する (認可コードは Pod 間で共有、F-10)。共有ストア障害時の挙動は `server_error` / `temporarily_unavailable` で表現する                                                                         |
| パフォーマンス | M1.2 では計測対象外。認可コードストアへのアクセスがリクエストごとに発生するため、ストア選択時にレイテンシを考慮する (Q1)                                                                                                                                             |
| 保守性         | `/authorize` の handler 層と認可コードストア層を分離する。M2.2 の rotation / 不正再使用検知導入時にストア層のみを拡張すればよい構造とする                                                                                                                            |
| ポータビリティ | OS / アーキテクチャ依存コードを書かない。乱数生成は Go 標準ライブラリ (`crypto/rand`) を用いる                                                                                                                                                                       |
| 検証性         | F-1〜F-20 を Go 単体テストおよび統合テスト (実機 K8s 不要) で検証可能。CI 上で同等のテストが green になる                                                                                                                                                            |
| 観測性         | 認可リクエストの受付・成否・エラーコードを構造化ログで記録し、`request_id` で追跡できる (監査ログ本体は M7.1)                                                                                                                                                        |

## 認可要件

本マイルストーンはロール別認可制御を扱わない (F-17)。`/authorize` はエンドユーザーの OAuth2 認可フロー (RP への認可委譲) であり、`docs/context/authorization/matrix.md` のロール × 機能セルの対象外とする。

ユーザー確認 (2026-05-31): **M1.2 では認可マトリクスを更新しない**。ロール認可はロール導入時 (M3.x) にマスターへ反映する。

認可マスター (`docs/context/authorization/matrix.md`) は機能 × ロールが TBD であり、本マイルストーンでは**該当セルを追加しない**(対象外宣言のみ)。`/authorize` のエンドユーザー認証は固定 (F-15) で、ロール × 機能の認可制御は行わない (F-17)。よってマスター差分は発生せず、マスター変更の正規フロー (ADR 起票等) も不要。M3.x でロールを導入する際に、改めてマスターへ登録する。

## スコープ外 (含まないこと)

- トークンエンドポイント `/token` の実装 / `code_verifier` 照合 → M1.3
- UserInfo エンドポイント `/userinfo` の実装 → M1.4
- ID Token 生成・署名 (`nonce` claim 反映含む) → M1.3
- ユーザー DB / ローカルログイン画面 / パスワード認証 → M3.1 / M3.2
- Consent (同意) 画面 → M3.3
- クライアント登録 DB + 管理 API → M2.1
- 認可コードの rotation / 単回使用後の永続的失効管理 → M2.2
- 上流 IdP との連携 (id-core が RP) → M4.x
- 運用補強系 (監査ログ・refresh token family 失効・不正再使用検知) → Phase 7 (M7.x)

## 検討事項 (設計フェーズで論点化)

| #   | 論点                                                                                                                    | 第一候補                                                                                                   |
| --- | ----------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| Q1  | 認可コードの共有ストア技術選択 (DB 先行 / Redis 等) と TTL・一意性・単回使用の扱い                                      | DB 先行 (M0.3 基盤に最小テーブル)。TTL は OAuth2 推奨に従い短命 (例: 60〜600 秒)。rotation 本格対応は M2.2 |
| Q2  | 固定クライアント設定の形式 (env / config ファイル / 複数許可) と保持項目 (`client_id` / `redirect_uri` 群 / 許可 scope) | config ファイル + env override。M2.1 の DB スキーマと整合する項目構成                                      |
| Q3  | 固定ユーザーの定義方法 (`sub` 値・設定キー)                                                                             | env / config で単一の固定 `sub`。M3.1 の共通 ID 体系と整合する形式                                         |
| Q4  | リダイレクト不可時の直接エラー表示の形式 (HTML / JSON) と M0.2 エラー規約との接続方法                                   | M0.2 のエラー規約に準拠。ブラウザ向けは最小限の HTML、機械向けは JSON を検討                               |
| Q5  | 認可コードの形式・エントロピー (乱数長・エンコード)                                                                     | `crypto/rand` ベースの十分なエントロピー (例: 256bit) を base64url                                         |
| Q6  | `redirect_uri` のスキーム制約 (`https` 必須 / loopback `http` 許可 / カスタムスキーム)                                  | 公開クライアント想定。`CORE_ENV` に応じた制約を #32 Q13 の issuer 正規化方針と整合させる                   |

> 当初検討事項にあった「HTTP メソッド (GET/POST)」と「`openid` 以外 scope の扱い」は、OpenID Connect Core 1.0 の MUST/SHOULD に基づき要求として確定したため検討事項から除外した (GET+POST 両対応 = F-1、未対応 scope は無視 = F-3)。

## 未決事項

要求レベルの未決事項なし。技術選択に関わる論点は上記「検討事項 (設計フェーズで論点化)」として設計フェーズへ明示的に委譲する。

## 要求フェーズ状況

| フェーズ              | 状態   | 備考                                                                                                                                                                                                  |
| --------------------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1. ドラフト           | 完了   | 2026-05-31、Issue #43 起点 + Codex レビュー (HIGH=4 / MEDIUM=4 / LOW=2) 反映済み                                                                                                                      |
| 2. 認可マトリクス突合 | 完了   | 2026-05-31、ロール認可は M1.2 対象外 (F-17)、マスター更新不要をユーザー確認                                                                                                                           |
| 3. 未決事項解決       | 完了   | 2026-05-31、要求レベル未決ゼロ。技術選択は検討事項 Q1〜Q6 として設計へ委譲                                                                                                                            |
| 4. レビュー           | 完了   | 2026-05-31、Codex レビュー 3 回実施。ドラフト (HIGH=4/MEDIUM=4/LOW=2) → 反映後再レビュー (HIGH=3/MEDIUM=3/LOW=1) → 3 回目 (CRITICAL=0/HIGH=0/MEDIUM=0/LOW=1) を全件反映。LOW 1 件 (Q 表記) も修正済み |
| 5. 公開ゲート         | 通過   | 2026-05-31 `/requirements-validate` 合格 + Codex 公開ゲート (CRITICAL=0 / HIGH=0 / MEDIUM=0) 通過。`/requirements-publish` へ                                                                         |
| 6. 公開               | 公開済 | 2026-05-31、Issue #43 本文同期 + `状態:設計着手OK` 付与。設計担当は `/spec-pickup 43` で引き取り可                                                                                                    |

## 公開記録

- 公開日時: 2026-05-31
- Issue: https://github.com/mktkhr/id-core/issues/43
- ゲート結果:
  - validate: 合格
  - CRITICAL: 0 / HIGH: 0 / MEDIUM: 0 (Codex レビュー 3 回目)

## 関連

- 親 Phase: Phase 1 (OIDC OP 最小コア)
- 先行マイルストーン: M1.1 (#32: Discovery + JWKS), M0.3 (#21: DB 接続 + マイグレーション基盤), M0.2 (#7: ログ・エラー・request_id 規約)
- 後続マイルストーン: M1.3 (`/token`) → M1.4 (`/userinfo`) → M1.5 (E2E with go-react RP)
- 関連スキル: `/backend-security` (OIDC OP セキュリティ / PKCE), `/backend-architecture`, `/backend-api-endpoint`
- 参照仕様:
  - OpenID Connect Core 1.0 (authorization_code フロー)
  - RFC 6749 (OAuth 2.0 Authorization Framework, §4.1 Authorization Code Grant)
  - RFC 7636 (Proof Key for Code Exchange, PKCE)
  - RFC 6750 (Bearer Token Usage, 参考)

## 変更履歴

| 日付       | 変更内容                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| ---------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 2026-05-31 | 起票 (Issue #43 起点)。M1.1 (#32) 完了を受けて M1.2 着手。ユーザー確認で PKCE=`S256` 必須 / `nonce`=任意 (受信時コード束縛) / 認可コードは HPA 複数 Pod 成立必須 (共有ストア) / 認可マトリクス M1.2 更新なし を確定                                                                                                                                                                                                                                                                                                                                                                                                                 |
| 2026-05-31 | ドラフトに対する Codex レビュー (CRITICAL=0 / HIGH=4 / MEDIUM=4 / LOW=2) を全件反映: エラーコード最低セット + リダイレクト禁止条件 (F-12 / F-13)、PKCE 責務を `/authorize` と `/token` で分離 (F-6)、HPA 複数 Pod 成立を非機能要件化 (F-10)、パラメータ必須/任意・欠落時挙動 (F-4〜F-7 / F-11)、`nonce` の M1.3 引き継ぎ (F-8)、`access_denied` 経路 (F-14)、認可マトリクス暫定方針明記 (F-17)、M0.2 エラー形式との接続 (F-12)、Out 粒度整理、成功時 `state` 条件明示 (F-7)                                                                                                                                                         |
| 2026-05-31 | 反映後文書の Codex 再レビュー (CRITICAL=0 / HIGH=3 / MEDIUM=3 / LOW=1) を全件反映: `/authorize` の `GET`+`POST` 両対応を明記 (F-1, OIDC Core §3.1.2.1 MUST)、`scope` エラー分岐を整理 (欠落=`invalid_request` / `openid` 不在=`invalid_scope` / 未対応 scope は無視、F-3)、`client_id` 欠落=`invalid_request` 直接エラーに修正 (F-4)、認可コードの `client_id`/`redirect_uri`/PKCE/`nonce`/`sub` 束縛を明記 (F-10, RFC 6749 §4.1.2)、エラーコード対応表を新設し F-11 を表参照に変更、認可要件表を削除し対象外宣言のみに、Out 粒度を Phase 7 に集約、検討事項を Q1〜Q6 にリナンバリング (旧 GET/POST・scope 論点を F 要件化して除外) |
| 2026-05-31 | Codex 3 回目レビュー (CRITICAL=0 / HIGH=0 / MEDIUM=0 / LOW=1) で公開ゲート通過。LOW 1 件 (フェーズ状況表の検討事項旧表記 `Q1〜Q8`) を `Q1〜Q6` に修正。`/requirements-publish` で Issue #43 本文を同期し `状態:設計着手OK` に遷移。要求フェーズ完了                                                                                                                                                                                                                                                                                                                                                                                 |
| 2026-05-31 | 設計フェーズの Codex レビュー指摘 (HIGH) を受け F-19 を技術的に修正: `code_challenge` は PKCE 仕様上の公開値 (challenge は公開、`code_verifier` が秘密) のため秘匿対象から除外。秘匿対象を `code` / `code_verifier` 等に明確化。認可記述・スコープの変更なし (ログ redact 方針の正確化のみ) のため Issue 本文の再同期は不要                                                                                                                                                                                                                                                                                                         |
