# 設計 #32: OIDC Discovery + JWKS エンドポイントを core/ に導入

- 関連要求: [`requirements/32/index.md`](../../requirements/32/index.md)
- 関連 Issue: [#32](https://github.com/mktkhr/id-core/issues/32)
- マイルストーン: [M1.1: OIDC Discovery + JWKS エンドポイント](https://github.com/mktkhr/id-core/milestone/4)
- 状態: 着手中
- 起票日: 2026-05-02
- 最終更新: 2026-05-03

## 関連資料

- 要求文書: [`requirements/32/index.md`](../../requirements/32/index.md)
- 認可マトリクス (正本): [`../../context/authorization/matrix.md`](../../context/authorization/matrix.md) — 公開エンドポイントのため対象セルなし
- アーキテクチャ: [`../../context/app/architecture.md`](../../context/app/architecture.md)
- 規約: [`../../context/backend/conventions.md`](../../context/backend/conventions.md) — 本マイルストーンで OIDC OP 規約節を新設
- 規約 (パターン): [`../../context/backend/patterns.md`](../../context/backend/patterns.md)
- レジストリ: [`../../context/backend/registry.md`](../../context/backend/registry.md) — 本マイルストーンで `CORE_OIDC_*` 系を追加
- テスト規約: [`../../context/testing/backend.md`](../../context/testing/backend.md)
- 関連 ADR: 必要に応じて起票 (現状なし)
- 参照仕様:
  - OpenID Connect Discovery 1.0
  - RFC 8414 (OAuth 2.0 Authorization Server Metadata)
  - RFC 7517 (JSON Web Key)
  - RFC 7518 (JSON Web Algorithms)
  - OpenID Connect Core 1.0

## 編集ルール (本ファイル限定)

- 本ファイル内では `@` 記号を裸で書かない (`@v1` / `@master` / `@user` 等)。GitHub 自動メンション混入を防ぐため、必ずバッククォートで囲むか別表現に言い換える
- 既存の規約は [`.rulesync/rules/pr-review-policy.md`](../../../.rulesync/rules/pr-review-policy.md) §5 を参照

## 要件の解釈

### スコープ

id-core を OIDC OP として外部 RP が認識・接続できる最小骨格。公開対象は **メタデータ + 公開鍵** の 2 経路のみ。実エンドポイント (`/authorize` / `/token` / `/userinfo`) の本実装は M1.2 以降。

### 主要な決定事項 (要求文書からの引き継ぎ)

| 区分                    | 決定                                                                                                      |
| ----------------------- | --------------------------------------------------------------------------------------------------------- |
| 鍵フォーマット          | PEM PKCS#8 (Q1)                                                                                           |
| dev 鍵生成              | Go 標準 `crypto/rsa` + `crypto/x509` (`core/cmd/devkeygen/`、Q2)                                          |
| JWK 操作ライブラリ      | `github.com/lestrrat-go/jwx/v3` (Q3)                                                                      |
| Discovery Cache-Control | 既定 `0` で `no-cache, must-revalidate`、`>0` で `public, max-age=<N>, must-revalidate` (env で切替、Q15) |
| JWKS Cache-Control      | `public, max-age=300, must-revalidate` (既定)、env で 0〜86400 override (Q4)                              |
| keystore I/F            | `KeySet { Active(ctx), Verifying(ctx) }`、M1.1 は `staticKeySet` で 1 鍵保持 (Q10)                        |
| `CORE_ENV`              | strict 3 値 (`prod` / `staging` / `dev`)、不正値 / 空 / unset で起動失敗 (Q7)                             |
| issuer 正規化           | `https://` 必須 (`CORE_ENV=dev` のみ `http://` 許可)、末尾スラッシュ strip (Q13)                          |
| JWKS path               | `/jwks` (拡張子なし、Q9)                                                                                  |
| middleware              | M0.2 D1 順序踏襲: `request_id → access_log → recover → handler` (Q14)                                     |
| token endpoint 認証方式 | M1.1 広告は `client_secret_basic` のみ (Q12)                                                              |
| 未実装 endpoint         | HTTP 503 + 機械可読 JSON `{"error":"endpoint_not_implemented","available_at":"<milestone>"}` (F-23)       |
| 鍵更新運用              | M1.1 は非サポート、Pod 全停止 → 再起動 (F-24)、zero-downtime rotation は M2.x                             |
| 複数 Pod ガード         | アプリ側 WARN ログのみ、強制は Helm/manifest 側責任 (Q5、実 sample は M1.5)                               |
| 署名アルゴリズム        | RS256 のみ広告 (F-12)                                                                                     |

### 既存実装からの統合点

M0.3 までで確立済の以下を起点にする:

- `core/cmd/core/main.go`: 起動シーケンス (実装実態 = `bootstrap (config → logger → event_id) → ctx (WithEventID) → db.Open → AssertClean → server.New → ListenAndServe`)。本マイルストーンで **`bootstrap` 後・`db.Open` 前後 (詳細は論点) に keystore 初期化を挿入** (F-15)
- `core/internal/config/`: 環境変数読み込み (`Config { Port, Database }`)。**`CORE_OIDC_*` 系と `CORE_ENV` の strict 検証を追加**
- `core/internal/server/`: chi router + middleware チェーン。**`/health/*` と並ぶ無認可層に `/.well-known/openid-configuration` と `/jwks` を追加**
- `core/internal/apperror/`: `CodedError` (M0.2、`Code()` は SCREAMING_SNAKE_CASE 推奨、既存例 `INTERNAL_ERROR` / `INVALID_PARAMETER` / `NOT_FOUND`)。**OIDC エンドポイントの 503 / 4xx レスポンス body は `CodedError` 経由とするかを論点で確定** (内部 code 命名規約と OIDC RFC 6749 の error フィールド命名 (snake_case) の二重化が必要)
- `core/internal/middleware/`: D1 順序 (M0.2 確定)。OIDC 用に追加なし

### 新規追加されるパッケージ (案、論点で確定)

- `core/internal/keystore/`: 鍵読み込み + KeySet I/F + 起動時生成モード + kid 算出
- `core/internal/oidc/discovery/`: Discovery handler + メタデータ構築
- `core/internal/oidc/jwks/`: JWKS handler + JWK serialize + ETag
- `core/internal/oidc/notimpl/`: 未実装 endpoint の 503 stub handler (M1.2-1.4 で実装に差し替え)
- `core/cmd/devkeygen/`: dev 鍵生成 CLI

## 設計時の論点

| #      | 論点                                                                                                                                                                                                                                                                                                                            | 第一候補                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | 状態 |
| ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---- |
| 1      | パッケージ分割: 鍵管理層は `internal/keystore/` 単独配置 vs `internal/oidc/keystore/` ネスト配置のどちらにするか                                                                                                                                                                                                                | `internal/keystore/` 単独 (鍵管理は OIDC 以外でも将来再利用可能性あり、責務分割としても独立配置が素直)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     | 未決 |
| 2      | Discovery / JWKS handler の配置: `internal/oidc/discovery` + `internal/oidc/jwks` (機能粒度パッケージ) vs `internal/oidc/handler/` (フラット)                                                                                                                                                                                   | 機能粒度パッケージ (Package by Feature 規約踏襲、M1.2 以降の `/authorize` 等の追加でも一貫)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | 未決 |
| 3      | M1.2-1.4 で実装される endpoint (`/authorize` / `/token` / `/userinfo`) の 503 stub をどう差し替え可能にするか (差し替え動線設計)                                                                                                                                                                                                | `notimpl.Handler(milestone string)` のような共通ファクトリで宣言的に登録、後続マイルストーンで route 登録を上書きする (chi の `Route` で同 path を再宣言する形式)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | 未決 |
| 4      | ETag 計算アルゴリズム: Go 標準 `crypto/sha256` を JWKS / Discovery の決定的 JSON バイト列に直接適用 vs `jwx/v3` の deterministic serialize 経由で同等の値を得る                                                                                                                                                                 | Go 標準 `crypto/sha256` で 32 バイトのハッシュを取り、**先頭 16 バイトを base64url-no-padding** したものを strong ETag (例: `"abcd_xxxxxxxxxxxxxxx"`、22 文字) として返す。入力は jwx の決定的 marshal 結果 (JWKS) または独自 marshal の決定的バイト列 (Discovery)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | 未決 |
| 5      | dev 鍵生成 (`make dev-keygen`) の RSA 鍵長: 2048 vs 4096                                                                                                                                                                                                                                                                        | 2048 (大手 OP の標準値、生成・署名コストのバランス、F-12 の RS256 で十分な強度)。本番 K8s Secret に乗せる鍵は運用側選定なので開発鍵のみの判断                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | 未決 |
| 6      | subpath 配置 (`CORE_OIDC_ISSUER=https://example.com/id-core`) 時の Discovery `issuer` フィールド出力値: 末尾スラッシュ無 (`https://example.com/id-core`) vs 末尾スラッシュ付 (`https://example.com/id-core/`)                                                                                                                   | 末尾スラッシュ無 (Q13 の正規化方針 = strip と整合、ID Token `iss` claim との一致は OP の発行値に揃えるのが原則)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | 未決 |
| 7      | `Content-Type`: Discovery は `application/json` 固定で良いか / JWKS は `application/json` か `application/jwk-set+json` (RFC 7517 §8.5) か                                                                                                                                                                                      | Discovery: `application/json` (RFC 8414 §3 規定)。JWKS: `application/jwk-set+json` を第一、`application/json` を後方互換で許容 (RFC 7517 §8.5 / 多くの OP は `application/json` を返すため互換性を優先する判断もある — どちらを正にするかを決める)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | 未決 |
| 8      | 503 レスポンスの実装方式 + 内部 code と OIDC RFC 6749 `error` フィールドの命名規約の二重化: 内部 `apperror.CodedError.Code()` は SCREAMING_SNAKE_CASE 推奨 (M0.2 既存規約: `INTERNAL_ERROR` 等) だが、レスポンス body の `error` フィールドは OIDC RFC 6749 で snake_case (`endpoint_not_implemented` 等)。両者をどう分離するか | **(a) 内部 code = `ENDPOINT_NOT_IMPLEMENTED`** を `apperror.CodedError` に追加 (apperror 命名規約遵守)。**(b) HTTP レスポンス body の `error` フィールド = `endpoint_not_implemented`** は notimpl ハンドラ内で固定文字列として出力 (OIDC 標準準拠)。M1.2 以降の本実装で OIDC 標準エラー (`invalid_request` 等) を返す際にも同パターン (内部 code は SCREAMING_SNAKE_CASE、レスポンス body は snake_case) を適用。レスポンス書き込みは notimpl 専用の minimal writer (chi handler 内で `w.Header().Set("Content-Type","application/json")` + `w.WriteHeader(503)` + `json.NewEncoder(w).Encode(map[string]any{"error":"endpoint_not_implemented","available_at":"<milestone>"})`) とし、apperror.WriteJSON は使わない (apperror は `code/message/request_id` 形式で OIDC 標準と異なるため) | 未決 |
| 9      | keystore 初期化失敗時のエラーログ表現: 起動シーケンスで失敗した場合、`apperror.CodedError` を介すか / プレーン `error` で `logger.Error` するか                                                                                                                                                                                 | プレーン `error` (`fmt.Errorf` で wrap) で `logger.Error` する (起動失敗は HTTP に出ないため `CodedError` の付加情報 = `code/message/details` は不要)。M0.3 の既存パターン (`l.Error(ctx, "DB 接続に失敗しました", err)`) を踏襲。仮に `CodedError` を使う場合の code 候補は `OIDC_KEYSTORE_INIT_FAILED` (本論点で確定し規約書に明記)                                                                                                                                                                                                                                                                                                                                                                                                                                                      | 未決 |
| 10     | jwx/v3 の API 採用箇所: 公開鍵 → JWK 変換 / JWK Set serialize / kid 自動算出 (`jwk.AssignKeyID`) のどこまで使うか                                                                                                                                                                                                               | 公開鍵 → JWK 変換 = `jwk.Import` / `jwk.Set` 構築・marshal は jwx/v3、kid は **自前実装** (F-11 の決定的アルゴリズム = SHA-256 先頭 24 hex を満たす)、決定的シリアライズは jwx の `json.Marshal` がキー順序を保証するか確認しダメなら自前で固定キー順 marshal                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | 未決 |
| 11     | Discovery / JWKS handler の access_log 出力: `request_id` のみ vs handler 名 / endpoint 種別をフィールド追加するか                                                                                                                                                                                                              | M0.2 既存 access_log の `path` フィールドで十分判別可能なので追加フィールド不要 (規約踏襲)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | 未決 |
| 12     | endpoint 個別 override env (`CORE_OIDC_AUTHORIZATION_ENDPOINT` 等) の優先順位: 設定があれば issuer 由来の構築値より優先 / issuer 配下に強制                                                                                                                                                                                     | 設定があれば優先 (Discovery の `issuer` と endpoint の host が異なっても OIDC Discovery 1.0 上は許容、ただし RP の OIDC ライブラリで warning を出す実装あり)。F-3 にすでに「override 可能」と記載済のため明文化のみ                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | 未決 |
| 13     | `CORE_OIDC_KEY_FILE` の Permission ガード: 起動時に `0600` / `0400` などの mode をチェックするか / OS 任せにするか                                                                                                                                                                                                              | M1.1 は OS 任せ (K8s Secret マウントは `defaultMode` でマウントされ、Pod 内 process は read のみ可。アプリ側で `stat` して mode 警告を出すのは過剰)。本論点で確定し規約書に明記                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | 未決 |
| 14     | 統合テストにおける鍵経路: テスト時に temp file 経由で `CORE_OIDC_KEY_FILE` を渡す vs `CORE_OIDC_DEV_GENERATE_KEY=1` で代替                                                                                                                                                                                                      | 起動失敗テスト (`prod` + 鍵未指定) は env のみで完結。Discovery / JWKS の正常系統合テストは `dev` + `DEV_GENERATE_KEY=1` で十分 (テスト関数内で `crypto/rsa.GenerateKey` してメモリで完結する F-14 の方針)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | 未決 |
| ~~15~~ | ~~OpenAPI 仕様書 (`oapi-codegen` 由来) との関係~~                                                                                                                                                                                                                                                                               | ❌ **削除**: core/ には現時点で `oapi-codegen` / OpenAPI 仕様書が導入されていない (`core/openapi/` 不在、`core/Makefile` に oapi 系ターゲットなし)。core での OpenAPI 導入時期は別マイルストーンで判断。M1.1 範囲では論点として成立しないため除外                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | 削除 |

## 実装対象

| モジュール                         | 実装有無 | 内容                                                                                                                            |
| ---------------------------------- | :------: | ------------------------------------------------------------------------------------------------------------------------------- |
| `core/`                            |    ◯     | Discovery / JWKS handler、keystore 層、devkeygen CLI、config 拡張 (`CORE_OIDC_*` + `CORE_ENV` strict)、main.go 統合、規約書追記 |
| `examples/go-react/backend/`       |    ×     | 本マイルストーン対象外 (M1.5 で RP として接続)                                                                                  |
| `examples/go-react/frontend/`      |    ×     | 本マイルストーン対象外                                                                                                          |
| `examples/kotlin-nextjs/backend/`  |    ×     | 本マイルストーン対象外                                                                                                          |
| `examples/kotlin-nextjs/frontend/` |    ×     | 本マイルストーン対象外                                                                                                          |
| データベース                       |    ×     | M1.1 は鍵管理 + メタデータ公開のみで DB を使わない                                                                              |

## DB 設計

**本マイルストーン対象外。**

クライアント登録は M2.1、認可コード / トークン保存は M1.2-1.3 で扱う。M1.1 で導入する config (環境変数のみ) と keystore (ファイル / メモリ保持のみ) は永続化層を必要としない。

## API 設計

### エンドポイント一覧

| Method | Path                                | 認証                            | レスポンス                               | 備考                |
| ------ | ----------------------------------- | ------------------------------- | ---------------------------------------- | ------------------- |
| GET    | `/.well-known/openid-configuration` | 不要 (公開)                     | 200 + JSON Discovery metadata (RFC 8414) | F-1, F-2, F-3, F-4  |
| GET    | `/jwks`                             | 不要 (公開)                     | 200 + JSON JWKS (RFC 7517)               | F-5, F-6, F-21      |
| GET    | `/authorize`                        | 不要 (公開、未実装 stub)        | 503 + 機械可読 JSON                      | F-23、本実装は M1.2 |
| POST   | `/token`                            | (M1.3 で `client_secret_basic`) | 503 + 機械可読 JSON                      | F-23、本実装は M1.3 |
| GET    | `/userinfo`                         | (M1.4 で Bearer)                | 503 + 機械可読 JSON                      | F-23、本実装は M1.4 |

既存 (M0.3 まで):

| Method | Path            | 認証 | 備考                |
| ------ | --------------- | ---- | ------------------- |
| GET    | `/health/live`  | 不要 | M0.3                |
| GET    | `/health/ready` | 不要 | M0.3 (DB ping 含む) |

### Discovery メタデータ (`GET /.well-known/openid-configuration`)

レスポンス例 (`CORE_OIDC_ISSUER=https://id.example.com`):

```json
{
  "issuer": "https://id.example.com",
  "authorization_endpoint": "https://id.example.com/authorize",
  "token_endpoint": "https://id.example.com/token",
  "userinfo_endpoint": "https://id.example.com/userinfo",
  "jwks_uri": "https://id.example.com/jwks",
  "response_types_supported": ["code"],
  "grant_types_supported": ["authorization_code"],
  "subject_types_supported": ["public"],
  "id_token_signing_alg_values_supported": ["RS256"],
  "scopes_supported": ["openid"],
  "token_endpoint_auth_methods_supported": ["client_secret_basic"]
}
```

### Discovery レスポンスヘッダ

| ヘッダ          | 値                                                                                                              | 由来        |
| --------------- | --------------------------------------------------------------------------------------------------------------- | ----------- |
| `Content-Type`  | `application/json`                                                                                              | RFC 8414 §3 |
| `Cache-Control` | env `CORE_OIDC_DISCOVERY_MAX_AGE`=0 → `no-cache, must-revalidate` / >0 → `public, max-age=<N>, must-revalidate` | F-1, Q15    |
| `ETag`          | strong ETag (例: `"sha256-base64url-16byte"`)                                                                   | F-1, F-21   |

### JWKS (`GET /jwks`)

レスポンス例 (1 鍵 RSA 2048):

```json
{
  "keys": [
    {
      "kty": "RSA",
      "use": "sig",
      "alg": "RS256",
      "kid": "<24 hex>",
      "n": "<base64url>",
      "e": "AQAB"
    }
  ]
}
```

### JWKS レスポンスヘッダ

| ヘッダ          | 値                                                                                 | 由来          |
| --------------- | ---------------------------------------------------------------------------------- | ------------- |
| `Content-Type`  | `application/jwk-set+json` (第一候補、論点 #7 で確定)                              | RFC 7517 §8.5 |
| `Cache-Control` | `public, max-age=<X>, must-revalidate` (X = env `CORE_OIDC_JWKS_MAX_AGE` 既定 300) | F-6, Q4       |
| `ETag`          | strong ETag (JWKS バイト列由来、決定的)                                            | F-6, F-21     |

### 未実装 endpoint レスポンス (503)

```http
HTTP/1.1 503 Service Unavailable
Content-Type: application/json
Cache-Control: no-store
```

```json
{
  "error": "endpoint_not_implemented",
  "available_at": "M1.2"
}
```

`available_at` は endpoint ごとに固定 (`/authorize`=`M1.2` / `/token`=`M1.3` / `/userinfo`=`M1.4`、F-23)。

> **HTTP `Retry-After` ヘッダは付与しない**。RFC 7231 §7.1.3 で値は HTTP-date または delta-seconds (整数秒) のみ許容され、`M1.2` のような文字列は規約違反。`available_at` (マイルストーン名) は機械可読 JSON body 経由で RP に伝える。RP 側のリトライ抑制はキャッシュさせない方針 (`Cache-Control: no-store`) とすることで、本実装デプロイ後の即時切替を担保する。

### エラーコード

OIDC OP の標準エラーコード (RFC 6749 / OIDC Core) は M1.2 以降で扱う。M1.1 で新規導入する code (論点 #8 で命名規約二重化を確定):

| 内部 code (apperror.CodedError, SCREAMING_SNAKE_CASE) | OIDC レスポンス body `error` フィールド (snake_case) | HTTP | 用途                                                                                                             |
| ----------------------------------------------------- | ---------------------------------------------------- | ---- | ---------------------------------------------------------------------------------------------------------------- |
| `ENDPOINT_NOT_IMPLEMENTED`                            | `endpoint_not_implemented`                           | 503  | F-23 の未実装 endpoint stub。レスポンス body は notimpl handler が直接書き込み (apperror.WriteJSON は経由しない) |
| `OIDC_KEYSTORE_INIT_FAILED` (任意採用、論点 #9)       | (なし、起動失敗で HTTP に出ない)                     | -    | 起動時 keystore 初期化失敗時のログ用 code (`logger.Error` 内で構造化)                                            |

> **命名規約二重化の理由**:
> 内部 code は M0.2 既存規約 (SCREAMING_SNAKE_CASE) に従う。OIDC レスポンス body の `error` フィールドは RFC 6749 §5.2 / OIDC Core §3.1.2.6 で snake_case (`invalid_request` 等) が標準のため、両者は別物として明示的に二重化する。M1.2 以降の本実装で OIDC 標準エラーコード (`invalid_request` / `invalid_grant` 等) を返す場合も同パターンを継承する。

### 環境変数 (新規追加)

| 環境変数                           | 必須 | 既定値                                 | 用途                                                                                                                   |
| ---------------------------------- | :--: | -------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| `CORE_ENV`                         |  ◯   | (なし、未設定で起動失敗)               | 環境識別子。`prod` / `staging` / `dev` のみ許容 (F-9, Q7)                                                              |
| `CORE_OIDC_ISSUER`                 |  ◯   | (なし)                                 | OP の論理識別子 URL。`prod`/`staging` で `https://` 必須、`dev` は `http://` 許可 (F-1, Q13)                           |
| `CORE_OIDC_KEY_FILE`               |  △   | (なし)                                 | PEM PKCS#8 秘密鍵ファイルパス。`CORE_ENV=prod` で必須、それ以外は `CORE_OIDC_DEV_GENERATE_KEY=1` で代替可能 (F-7, F-9) |
| `CORE_OIDC_DEV_GENERATE_KEY`       |  ×   | `0`                                    | `1` で起動時 RSA 鍵生成。`CORE_ENV=prod` では強制無効 (F-7)                                                            |
| `CORE_OIDC_KEY_ID`                 |  ×   | (自動算出: 公開鍵 SHA-256 先頭 24 hex) | kid 固定値 (F-11)                                                                                                      |
| `CORE_OIDC_JWKS_MAX_AGE`           |  ×   | `300`                                  | JWKS Cache-Control max-age 秒 (0〜86400) (F-6, Q4)                                                                     |
| `CORE_OIDC_DISCOVERY_MAX_AGE`      |  ×   | `0`                                    | Discovery Cache-Control max-age 秒 (0 → `no-cache`、>0 → `public, max-age=<N>`) (F-1, Q15)                             |
| `CORE_OIDC_AUTHORIZATION_ENDPOINT` |  ×   | issuer + `/authorize`                  | endpoint 個別 override (F-3)                                                                                           |
| `CORE_OIDC_TOKEN_ENDPOINT`         |  ×   | issuer + `/token`                      | endpoint 個別 override (F-3)                                                                                           |
| `CORE_OIDC_USERINFO_ENDPOINT`      |  ×   | issuer + `/userinfo`                   | endpoint 個別 override (F-3)                                                                                           |
| `CORE_OIDC_JWKS_URI`               |  ×   | issuer + `/jwks`                       | jwks_uri 個別 override                                                                                                 |

## 認可設計

本スコープのエンドポイントは全て **公開エンドポイント (認証不要、F-16)**。認可マトリクス (`docs/context/authorization/matrix.md`) には公開エンドポイントの行は存在せず、本設計書もマスターからのコピー対象なし。

| 機能                                    | 全ロール (未認証含む) | 備考                                |
| --------------------------------------- | --------------------- | ----------------------------------- |
| `GET /.well-known/openid-configuration` | o (公開、認証不要)    | 要求 F-16                           |
| `GET /jwks`                             | o (公開、認証不要)    | 要求 F-16                           |
| `GET /authorize` (503 stub)             | o (公開、認証不要)    | M1.2 で本実装、認可は実装時に再設計 |
| `POST /token` (503 stub)                | o (公開、認証不要)    | M1.3 で `client_secret_basic`       |
| `GET /userinfo` (503 stub)              | o (公開、認証不要)    | M1.4 で Bearer                      |

> マスターと差分が出たら**即停止しユーザー判断を仰ぐ**。本マイルストーンでは差分なし (公開エンドポイントは元来マスター対象外)。

## フロー図 / シーケンス図

`/spec-diagrams` フェーズで起動シーケンス + Discovery 取得フロー + JWKS 取得フロー + 鍵 rotation 予告 (M2.x) を Mermaid で記述する。

(本フェーズは雛形のみ、後続フェーズで詳細化)

## テスト観点

`/spec-tests` フェーズで観点を網羅化。雛形:

### バックエンド単体

- Discovery handler: F-1〜F-4 のフィールド網羅、Cache-Control / ETag ヘッダ
- JWKS handler: F-5 のフィールド、F-6 ヘッダ、F-21 決定的シリアライズ
- keystore: PEM 読み込み (PKCS#8 正常系)、kid 自動算出 (F-11 決定論)、Active / Verifying I/F
- 503 stub handler (notimpl): F-23 の機械可読 JSON 形式 (`error`/`available_at` フィールド、Cache-Control: no-store、Retry-After 不在)、endpoint ごとの `available_at` 値 (M1.2/M1.3/M1.4)、内部 code = `ENDPOINT_NOT_IMPLEMENTED` と body フィールド = `endpoint_not_implemented` の二重化が崩れないこと
- config: `CORE_ENV` strict 検証 (F-9, Q7) / issuer 正規化 (Q13)

### バックエンド ContractTest (F-17)

issuer 5 ケース (Q8) のテーブル駆動:

1. 標準: `https://id.example.com`
2. subpath: `https://example.com/id-core`
3. 末尾スラッシュ: `https://example.com/id-core/` → strip
4. dev: `http://localhost:8080`
5. 非標準ポート: `https://id.example.com:9443`

### バックエンド統合 (M0.3 testutil/dbtest 流用)

- `CORE_ENV=prod` + `CORE_OIDC_KEY_FILE` 未設定で起動失敗 (F-9, F-20-c)
- `CORE_ENV=dev` + `CORE_OIDC_DEV_GENERATE_KEY=1` で起動成功 + Discovery / JWKS 200 (F-20-a, F-20-d)
- 起動時 INFO ログに kid / fingerprint / アルゴリズム / `CORE_ENV` 値が含まれる (F-18, 観測性)
- `Verifying()` が Active 鍵 1 件のみ返す (M1.1 の staticKeySet)

### E2E (本マイルストーン対象外、M1.5 で RP 接続)

M1.5 で go-react RP から `go-oidc` ライブラリ初期化 → Discovery / JWKS 自動取得 → kid 一致確認。

### セキュリティテスト

- 秘密鍵 / PEM フルダンプ / RSA modulus / exponent がログに出力されないこと (F-18 redact)
- `CORE_ENV=prod` で `CORE_OIDC_DEV_GENERATE_KEY=1` を指定しても起動失敗 (F-9)
- M0.3 の F-10 redact 方針が継続適用される (M0.3 conventions §F-10 / patterns)

## 既存資料からの差分

`/spec-track` フェーズで詳細化。本マイルストーンで更新される `context/` ファイル:

### `docs/context/backend/conventions.md`

新設「OIDC OP 規約」節 (F-19、最低 6 項目):

1. issuer URL 規約 (https 必須 + dev 例外、末尾スラッシュ strip)
2. 環境変数一覧 (`CORE_OIDC_*` / `CORE_ENV`)
3. 鍵保管方式 (本番 = K8s Secret マウント / dev = devkeygen) + kid と fingerprint の関係 (F-11 / F-18 同値)
4. dev 鍵生成モードの単一 Pod 制約 (Helm/manifest 側で `replicas: 1` 強制、実 sample は M1.5)
5. 開発者向け運用手順 (`make dev-keygen` / docker compose / make run)
6. M1.1 の鍵更新運用制約 (F-24: Pod 全停止 → 起動) と M2.x の overlap window 予告 (F-22)

### `docs/context/backend/registry.md`

- パッケージ追加: `internal/keystore/`, `internal/oidc/discovery/`, `internal/oidc/jwks/`, `internal/oidc/notimpl/`
- CLI 追加: `cmd/devkeygen/`
- 環境変数追加: 上記 11 件 (`CORE_OIDC_*` + `CORE_ENV`)
- `apperror.CodedError` の code 追加 (論点 #8 / #9 で確定): `ENDPOINT_NOT_IMPLEMENTED` (必須)、`OIDC_KEYSTORE_INIT_FAILED` (論点 #9 で採用された場合のみ)。OIDC レスポンス body の `error` フィールド命名規約 (snake_case) と内部 code (SCREAMING_SNAKE_CASE) の二重化方針

### `docs/context/backend/patterns.md`

- 起動時生成モード = メモリ保持限定パターン
- 公開エンドポイント (`/.well-known/*` / `/jwks`) の middleware チェーン適用方針 (D1 順序踏襲、Q14)
- 503 stub による未実装 endpoint のフォワード互換 (F-23、handler 差し替え動線、論点 #3 で確定)

### `docs/context/testing/backend.md`

- env 切替テストパターン (`CORE_ENV=prod` 起動失敗の検証手法)
- ContractTest テーブル駆動例 (Q8 の 5 ケース)

### `core/Makefile`

- `dev-keygen` ターゲット追加 (F-13)
- `run` ターゲットの `CORE_ENV ?= dev` デフォルト (F-9, Q7)

### `.gitignore`

- `core/dev-keys/` を追加 (F-14、秘密鍵がコミットされないこと)

## 設計フェーズ状況

| フェーズ               | 状態   | 備考                                                                                                    |
| ---------------------- | ------ | ------------------------------------------------------------------------------------------------------- |
| 1. 起票                | 完了   | 2026-05-02、Issue #32 起点、要求 #32 確定情報引き継ぎ                                                   |
| 2. 下書き              | 完了   | 2026-05-02、`/spec-create` で雛形生成                                                                   |
| 3. 規約確認            | 未着手 | `/check-convention` で M0.3 までの規約 (config / middleware / apperror / Makefile / .gitignore 等) 調査 |
| 4. 論点解決            | 未着手 | 論点 #1〜#15 を `/spec-resolve` で順次解決                                                              |
| 5. DB 設計             | 対象外 | 永続化なし                                                                                              |
| 6. API 設計            | 雛形済 | エンドポイント / レスポンス / ヘッダ / env を記載済、論点解決後に追補                                   |
| 7. 認可設計            | 完了   | 公開エンドポイントのためマスター対象外、本書に既述                                                      |
| 8. 図                  | 未着手 | `/spec-diagrams` で起動シーケンス + Discovery / JWKS フロー + M2.x rotation 予告                        |
| 9. テスト設計          | 雛形済 | `/spec-tests` で観点を網羅化 (data-testid は本スコープ外)                                               |
| 10. 差分整理           | 雛形済 | `/spec-track` で context 各ファイルへの差分を最終化                                                     |
| 11. 実装プロンプト生成 | 未着手 | `/spec-prompts` で P1〜P4 のプロンプトを `prompts/` 配下に生成                                          |

## 変更履歴

| 日付       | 変更内容                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| ---------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 2026-05-02 | 起票 (Issue #32 / 要求 #32 確定情報引き継ぎ)、`/spec-create` で雛形生成。要件解釈・実装対象・API 設計雛形・認可設計 (公開エンドポイントのため対象外)・テスト観点雛形・差分整理雛形を記載。論点 15 件を初期登録 (#1〜#15)                                                                                                                                                                                                                                                                                                                                                                 |
| 2026-05-03 | `/doc-review` セルフレビュー (HIGH=3 / MEDIUM=3 / LOW=1) を反映: (HIGH 1) `apperror.AppError` → `apperror.CodedError` に表記統一、(HIGH 2) 503 stub の `Retry-After` ヘッダ削除 (RFC 7231 違反、`Cache-Control: no-store` に置換)、(HIGH 3) 内部 code (SCREAMING_SNAKE_CASE) と OIDC レスポンス `error` フィールド (snake_case) の二重化を論点 #8 で明示、(MEDIUM 1) 論点 #9 を `kind` 不在踏まえてプレーン error 経路に修正、(MEDIUM 2) 論点 #15 (OpenAPI) 削除、(MEDIUM 3) 起動シーケンス記述を実装準拠に修正、(LOW 1) ETag 仕様明文化 (SHA-256 先頭 16 バイト → base64url-no-padding) |
