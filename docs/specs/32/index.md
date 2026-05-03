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

| #          | 論点                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | 第一候補                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | 状態                        |
| ---------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------- |
| 1          | パッケージ分割: 鍵管理層は `internal/keystore/` 単独配置 vs `internal/oidc/keystore/` ネスト配置のどちらにするか                                                                                                                                                                                                                                                                                                                                                                                                                                                       | ✅ **A 確定**: `core/internal/keystore/` 単独配置。理由 = 将来 M4.x の上流 IdP RP 連携 / その他 HMAC 署名等での再利用余地、依存方向 (oidc → keystore の一方向) の素直さ                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | 確定                        |
| 2026-05-03 | 論点 #2 確定: OIDC handler 配置 = **A (機能粒度パッケージ: `oidc/discovery/` + `oidc/jwks/` + `oidc/notimpl/`)**。Package by Feature 規約遵守、論点 #3 の差し替え動線と整合                                                                                                                                                                                                                                                                                                                                                                                            |
| 2026-05-03 | 論点 #3 確定: 503 stub 差し替え動線 = **A (`notimpl.Handler(milestone)` 共通ファクトリ + chi route 登録差し替え)**。M1.2 以降の本実装デプロイ時に server.go の route 登録 1 行を置き換える形                                                                                                                                                                                                                                                                                                                                                                           |
| 2026-05-03 | 論点 #4 確定: ETag 計算 = **Go 標準 `crypto/sha256` (32B) → 先頭 16B → `base64.RawURLEncoding` (no-padding) → strong ETag (24 文字、引用符込み)**。jwx に ETag 計算機能なし、標準ライブラリのみで完結                                                                                                                                                                                                                                                                                                                                                                  |
| 2026-05-03 | 論点 #5 確定: dev RSA 鍵長 = **2048 bit 固定** (`-bits` フラグなし)。dev 体験優先、業界標準 (Google / Microsoft / Auth0)                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| 2026-05-03 | 論点 #16 新規追加 + 確定: 本番鍵の責任分界 = **α (アプリは鍵をロードするだけ、本番鍵生成は id-core スコープ外)**。任意 bit 数の RSA 鍵を受け入れ (鍵長透過)。規約書 (F-19) にガイダンスのみ記載 (推奨 2048-4096 bit、>4096 bit は HTTP timeout 接近リスク)。テスト追加: RSA 1024/2048/3072/4096 bit の鍵長透過テスト                                                                                                                                                                                                                                                   |
| 2026-05-03 | 論点 #6 確定: subpath 配置時 Discovery `issuer` フィールド = **A (末尾スラッシュ無)**。Q13 strip と整合、ID Token `iss` claim と完全一致。Discovery URL は OpenID Connect Discovery 1.0 §4 に従い `<issuer>/.well-known/openid-configuration` (path 配下)                                                                                                                                                                                                                                                                                                              |
| 2026-05-03 | 論点 #7 確定: Content-Type = **β (Discovery: `application/json` MUST、JWKS: `application/json` 大手準拠 = Google/Microsoft/Okta)**。`application/jwk-set+json` は HTTP プロキシ / WAF の互換性リスク回避のため不採用                                                                                                                                                                                                                                                                                                                                                   |
| 2026-05-03 | 論点 #8 / #9 状態確定 (`/doc-review` で実体反映済): #8 = 内部 code (SCREAMING_SNAKE_CASE = `ENDPOINT_NOT_IMPLEMENTED`) と OIDC レスポンス body `error` フィールド (snake_case = `endpoint_not_implemented`) を分離、notimpl 専用 minimal writer で出力。#9 = keystore 初期化失敗はプレーン `error` で `logger.Error` (M0.3 既存パターン踏襲)                                                                                                                                                                                                                           |
| 2026-05-03 | 論点 #10 確定 (Codex セカンドオピニオン反映): jwx/v3 採用範囲 = a (公開鍵→JWK), b (Set 構築), c (marshal), e (PEM ロード = 標準) は jwx / 標準、d (kid) は **自前** (F-11 = DER SHA-256 先頭 24 hex、RFC 7638 thumbprint 非準拠を規約書に明記、ログ表記は "kid"/"fingerprint" に統一)。Codex HIGH 1 反映: jwx バージョン go.mod 固定 + golden / 契約テスト + ETag 安定性テスト追加。Codex MEDIUM 2 反映: PKCS#1 拒否 / encrypted PEM 非対応 / 短鍵 WARN を仕様化。Codex LOW 2 反映: `alg=RS256`/`kty=RSA`/`use=sig` 明示セット、private 成分が JWKS に出ないテスト追加 |
| 2026-05-03 | 論点 #11 確定 (Codex セカンドオピニオン反映、A 維持): access_log は M0.2 既存フィールドのみ、追加なし。将来追加候補 (`remote_ip` / `user_agent` / `host` / `scheme`) は別マイルストーンで検討                                                                                                                                                                                                                                                                                                                                                                          |
| 2026-05-03 | 論点 #12 確定: endpoint 個別 override env = **A (設定があれば優先)**。F-3 と整合、規約書に注意書き (異 host で RP ライブラリ warning の可能性、原則同一 host 推奨)                                                                                                                                                                                                                                                                                                                                                                                                     |
| 2026-05-03 | 論点 #13 確定: `CORE_OIDC_KEY_FILE` Permission ガード = **A (OS 任せ、アプリ側でチェックしない)**。本番は K8s Secret defaultMode、dev は devkeygen 側で `os.WriteFile(path, data, 0600)` を強制                                                                                                                                                                                                                                                                                                                                                                        |
| 2026-05-03 | 論点 #14 確定: 統合テストの鍵経路 = **A (テスト目的別使い分け)**。起動失敗系 = env のみ、Discovery / JWKS 正常系 = `DEV_GENERATE_KEY=1` (メモリ内 RSA)、ファイル経路異常系 = 単体テストで `crypto/rsa.GenerateKey` + `os.CreateTemp` 動的生成。F-14 (固定鍵コミット禁止) 担保                                                                                                                                                                                                                                                                                          |
| 2026-05-03 | 設計フェーズ状況更新: フェーズ 3 (規約確認) と 4 (論点解決) を完了。論点 16 件 (#1-14 確定 + #16 追加 + #15 削除) すべて確定。次は API 設計追補 → 図 → テスト → 差分整理 → プロンプト                                                                                                                                                                                                                                                                                                                                                                                  |
| 2          | Discovery / JWKS handler の配置: `internal/oidc/discovery` + `internal/oidc/jwks` (機能粒度パッケージ) vs `internal/oidc/handler/` (フラット)                                                                                                                                                                                                                                                                                                                                                                                                                          | ✅ **A 確定**: 機能粒度パッケージ (`oidc/discovery/` + `oidc/jwks/` + `oidc/notimpl/`)。Package by Feature 規約遵守 (M0.2/0.3 既存パッケージ群と一貫)。論点 #3 の差し替え動線 (`oidc/notimpl/` → 本実装パッケージへ置換) と整合                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | 確定                        |
| 3          | M1.2-1.4 で実装される endpoint (`/authorize` / `/token` / `/userinfo`) の 503 stub をどう差し替え可能にするか (差し替え動線設計)                                                                                                                                                                                                                                                                                                                                                                                                                                       | ✅ **A 確定**: `notimpl.Handler(milestone string)` 共通ファクトリ + chi の route 登録差し替え方式。M1.1 で `r.Get("/authorize", notimpl.Handler("M1.2"))` 等を登録し、M1.2 以降の本実装デプロイ時に route 登録 1 行を `authorize.Handler(...)` に置換する。`oidc/notimpl/` パッケージは M1.4 完了で全 import が消えた時点で削除                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | 確定                        |
| 4          | ETag 計算アルゴリズム: Go 標準 `crypto/sha256` を JWKS / Discovery の決定的 JSON バイト列に直接適用 vs `jwx/v3` の deterministic serialize 経由で同等の値を得る                                                                                                                                                                                                                                                                                                                                                                                                        | ✅ **A 確定**: Go 標準 `crypto/sha256` で 32 バイトハッシュ → **先頭 16 バイトを `base64.RawURLEncoding` (no-padding)** → ダブルクォートで囲んで strong ETag (例: `"abcd_efghIJKLmnopQRSt"`、24 文字 = 22 文字 + 引用符)。入力 = JWKS は jwx の決定的 marshal 結果、Discovery は自前の固定キー順 marshal バイト列。jwx に ETag 計算機能はないため標準ライブラリのみで完結。論点 #10 (jwx 採用範囲) と独立                                                                                                                                                                                                                                                                                                                                                                                                                              | 確定                        |
| 5          | dev 鍵生成 (`make dev-keygen`) の RSA 鍵長: 2048 vs 4096                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | ✅ **A 確定**: 2048 bit 固定 (`crypto/rsa.GenerateKey(rand.Reader, 2048)`)。dev 鍵生成 < 1 秒で開発体験優先、業界標準 (Google / Microsoft / Auth0 等)。`-bits` フラグは付けない (dev で 4096 を使う動機が薄い)。本番鍵長はアプリ側で強制しない (論点 #16)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | 確定                        |
| 6          | subpath 配置 (`CORE_OIDC_ISSUER=https://example.com/id-core`) 時の Discovery `issuer` フィールド出力値: 末尾スラッシュ無 (`https://example.com/id-core`) vs 末尾スラッシュ付 (`https://example.com/id-core/`)                                                                                                                                                                                                                                                                                                                                                          | ✅ **A 確定**: 末尾スラッシュ無 (`https://example.com/id-core`)。Q13 の正規化方針 = strip と整合、ID Token `iss` claim と完全一致。Discovery URL は OpenID Connect Discovery 1.0 §4 に従い `<issuer>/.well-known/openid-configuration` (path 配下) に配置 (規約書に明記)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | 確定                        |
| 7          | `Content-Type`: Discovery は `application/json` 固定で良いか / JWKS は `application/json` か `application/jwk-set+json` (RFC 7517 §8.5) か                                                                                                                                                                                                                                                                                                                                                                                                                             | ✅ **β 確定**: Discovery = `application/json` (RFC 8414 §3 MUST)。JWKS = **`application/json` (大手準拠)**。Google / Microsoft / Okta が事実上の標準として採用、RP ライブラリ (go-oidc / oidc-client-ts 等) は 100% 互換、一部 HTTP プロキシ / WAF で `application/jwk-set+json` を未知の Content-Type として警告 / ブロックするケース回避。RFC 7517 §8.5 でも `application/json` 許容                                                                                                                                                                                                                                                                                                                                                                                                                                                 | 確定                        |
| 8          | 503 レスポンスの実装方式 + 内部 code と OIDC RFC 6749 `error` フィールドの命名規約の二重化: 内部 `apperror.CodedError.Code()` は SCREAMING_SNAKE_CASE 推奨 (M0.2 既存規約: `INTERNAL_ERROR` 等) だが、レスポンス body の `error` フィールドは OIDC RFC 6749 で snake_case (`endpoint_not_implemented` 等)。両者をどう分離するか                                                                                                                                                                                                                                        | **(a) 内部 code = `ENDPOINT_NOT_IMPLEMENTED`** を `apperror.CodedError` に追加 (apperror 命名規約遵守)。**(b) HTTP レスポンス body の `error` フィールド = `endpoint_not_implemented`** は notimpl ハンドラ内で固定文字列として出力 (OIDC 標準準拠)。M1.2 以降の本実装で OIDC 標準エラー (`invalid_request` 等) を返す際にも同パターン (内部 code は SCREAMING_SNAKE_CASE、レスポンス body は snake_case) を適用。レスポンス書き込みは notimpl 専用の minimal writer (chi handler 内で `w.Header().Set("Content-Type","application/json")` + `w.WriteHeader(503)` + `json.NewEncoder(w).Encode(map[string]any{"error":"endpoint_not_implemented","available_at":"<milestone>"})`) とし、apperror.WriteJSON は使わない (apperror は `code/message/request_id` 形式で OIDC 標準と異なるため)                                             | ✅ 確定 (doc-review 反映済) |
| 9          | keystore 初期化失敗時のエラーログ表現: 起動シーケンスで失敗した場合、`apperror.CodedError` を介すか / プレーン `error` で `logger.Error` するか                                                                                                                                                                                                                                                                                                                                                                                                                        | プレーン `error` (`fmt.Errorf` で wrap) で `logger.Error` する (起動失敗は HTTP に出ないため `CodedError` の付加情報 = `code/message/details` は不要)。M0.3 の既存パターン (`l.Error(ctx, "DB 接続に失敗しました", err)`) を踏襲。仮に `CodedError` を使う場合の code 候補は `OIDC_KEYSTORE_INIT_FAILED` (本論点で確定し規約書に明記)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | ✅ 確定 (doc-review 反映済) |
| 10         | jwx/v3 の API 採用箇所: 公開鍵 → JWK 変換 / JWK Set serialize / kid 自動算出 (`jwk.AssignKeyID`) のどこまで使うか                                                                                                                                                                                                                                                                                                                                                                                                                                                      | ✅ **確定 (Codex セカンドオピニオン反映)**: a. 公開鍵→JWK 変換 = `jwk.Import` (jwx)、b. JWK Set 構築 = `jwk.NewSet()` + `set.AddKey()` (jwx)、c. JWK Set marshal = `json.Marshal(set)` (jwx の Marshaler)、d. kid 算出 = **自前** (F-11 = 公開鍵 DER SHA-256 先頭 24 hex、RFC 7638 thumbprint **非準拠**であることを規約書に明記、ログ表記は "kid" / "fingerprint" に統一し "thumbprint" は使わない)、e. PEM PKCS#8 ロード = `crypto/x509.ParsePKCS8PrivateKey` (標準) — c の決定論性を jwx 暗黙仕様に依存しすぎないため、(1) jwx をマイナー/パッチまで go.mod で固定 (依存自動更新を抑止)、(2) golden ファイルテスト + ETag 契約テスト追加、(3) jwx バージョン変更時の golden 更新は差分レビュー必須化。LOW: `alg=RS256` / `kty=RSA` / `use=sig` を明示セット、private 成分 (`d`/`p`/`q`/`dp`/`dq`/`qi`) が JWKS に出ないテストを追加 | 確定                        |
| 11         | Discovery / JWKS handler の access_log 出力: `request_id` のみ vs handler 名 / endpoint 種別をフィールド追加するか                                                                                                                                                                                                                                                                                                                                                                                                                                                     | ✅ **A 確定 (Codex 同意)**: 追加フィールドなし、M0.2 既存 access_log (`request_id` + `method` + `path` + `status` + `duration_ms`) のみ。`path` で endpoint 判別可能、ETag / Cache-Control はレスポンスヘッダで取れるためログ二重出力は K8s + fluent-bit/Loki でラベル爆発リスク。**将来追加候補** (別マイルストーンで検討、本マイルストーンでは追加しない): `remote_ip` (X-Forwarded-For 正規化) / `user_agent` / `host` (マルチドメイン時) / `scheme` (X-Forwarded-Proto)                                                                                                                                                                                                                                                                                                                                                            | 確定                        |
| 12         | endpoint 個別 override env (`CORE_OIDC_AUTHORIZATION_ENDPOINT` 等) の優先順位: 設定があれば issuer 由来の構築値より優先 / issuer 配下に強制                                                                                                                                                                                                                                                                                                                                                                                                                            | ✅ **A 確定**: 設定があれば優先 (`CORE_OIDC_<ENDPOINT>_ENDPOINT` env > issuer + path 構築値)。F-3 と整合 (override 可能、明文化のみ)。OIDC Discovery 1.0 上は issuer と endpoint host の不整合を許容するが、RP ライブラリで warning を出す実装あり。**規約書に注意書き**: 「異なる host を指定すると RP ライブラリで warning が発生する可能性あり、原則同一 host を推奨」                                                                                                                                                                                                                                                                                                                                                                                                                                                              | 確定                        |
| 13         | `CORE_OIDC_KEY_FILE` の Permission ガード: 起動時に `0600` / `0400` などの mode をチェックするか / OS 任せにするか                                                                                                                                                                                                                                                                                                                                                                                                                                                     | ✅ **A 確定**: OS 任せ (アプリ側で permission チェックしない)。K8s Secret マウントは `defaultMode` で read-only、本番側は K8s レイヤの責任。`make dev-keygen` 側で `os.WriteFile(path, data, 0600)` を強制 (devkeygen 実装で担保)。規約書に明記: 「id-core は鍵ファイルの permission を検証しない、運用側 (K8s Secret defaultMode / dev-keygen) で 0600 を担保」                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | 確定                        |
| 14         | 統合テストにおける鍵経路: テスト時に temp file 経由で `CORE_OIDC_KEY_FILE` を渡す vs `CORE_OIDC_DEV_GENERATE_KEY=1` で代替                                                                                                                                                                                                                                                                                                                                                                                                                                             | ✅ **A 確定**: テスト目的別使い分け。(1) 起動失敗テスト (`prod` + 鍵未指定): env のみ、鍵不要。(2) Discovery / JWKS 正常系統合テスト (`dev` + `DEV_GENERATE_KEY=1`): 起動時生成モードでメモリ内 RSA 鍵、ファイル不要。(3) ファイル経路の異常系 (PKCS#1 拒否 / encrypted PEM 拒否 / 短鍵 WARN 等、論点 #10 由来): 単体テスト側で `crypto/rsa.GenerateKey` + `os.CreateTemp` + `t.Cleanup` で動的 PEM 生成。F-14 (固定鍵コミット禁止) 担保 + テスト責務分離                                                                                                                                                                                                                                                                                                                                                                              | 確定                        |
| ~~15~~     | ~~OpenAPI 仕様書 (`oapi-codegen` 由来) との関係~~                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | ❌ **削除**: core/ には現時点で `oapi-codegen` / OpenAPI 仕様書が導入されていない (`core/openapi/` 不在、`core/Makefile` に oapi 系ターゲットなし)。core での OpenAPI 導入時期は別マイルストーンで判断。M1.1 範囲では論点として成立しないため除外                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | 削除                        |
| 16         | 本番鍵の責任分界: id-core が本番鍵生成を規定するか / 運用側 (DevOps / SRE / KMS / HSM) に委ねるか                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | ✅ **α 確定**: アプリは「鍵をロードするだけ」。本番鍵生成は **id-core スコープ外** = 運用側 (openssl / cloud KMS / HSM 等) で生成する。id-core は **任意 bit 数の RSA 鍵を受け入れる** (鍵長透過、env / config で強制しない)。規約書 (F-19) にガイダンスのみ記載: 推奨は 2048 bit 以上 (RS256 下限)、4096 bit まで実用範囲、>4096 bit は署名時間が秒オーダーになり HTTP timeout 接近リスクあり。devkeygen は dev 専用 (F-13)、本番ツール化しない                                                                                                                                                                                                                                                                                                                                                                                       | 確定                        |

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
| `Content-Type`  | `application/json` (大手準拠 = Google/Microsoft/Okta、論点 #7 で β 確定)           | RFC 7517 §8.5 |
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

本マイルストーンは UI / ユーザー操作なし (公開エンドポイント、F-16)、全て RP ライブラリ (例: go-oidc) からの自動取得。シーケンス図のみ記述する。

### 起動シーケンス (M1.1 で keystore 初期化を統合)

M0.3 までの起動順 (`bootstrap → ctx → db.Open → AssertClean → server`) に **keystore 初期化** を挿入する (F-15、論点 #1 / #16 確定済)。失敗時は `logger.Error` + `os.Exit(1)` (M0.3 同パターン、論点 #9 確定)。

```mermaid
sequenceDiagram
    autonumber
    participant Init as main()
    participant Boot as bootstrap()
    participant Cfg as config.Load
    participant Log as logger.Default
    participant DB as db.Open + AssertClean
    participant KS as keystore.Init
    participant Srv as server.New + ListenAndServe

    Init->>Boot: bootstrap()
    Boot->>Cfg: Load (CORE_ENV strict / CORE_OIDC_* / CORE_DB_*)
    alt env が prod/staging/dev 以外 or 必須欠落
        Cfg-->>Boot: error (起動失敗)
        Boot-->>Init: error
        Init-->>Init: stderr 出力 + os.Exit(1)
    end
    Boot->>Log: Default()
    Boot->>Boot: uuid.NewV7 で event_id 発番
    Boot-->>Init: cfg + logger + event_id
    Init->>Init: ctx = WithEventID(ctx, event_id)
    Init->>DB: Open(ctx) + AssertClean
    alt DB 接続失敗 or schema_migrations dirty
        DB-->>Init: error
        Init-->>Init: logger.Error + os.Exit(1)
    end
    Init->>KS: Init(ctx, cfg.OIDC)
    Note right of KS: 1. CORE_OIDC_KEY_FILE 優先<br/>2. なければ DEV_GENERATE_KEY=1 で<br/>   crypto/rsa.GenerateKey (2048 bit、メモリ保持)<br/>3. PEM = PKCS#8 のみ受け入れ<br/>   PKCS#1 / encrypted PEM はエラー<br/>4. kid = DER SHA-256 先頭 24 hex<br/>5. RFC 7638 thumbprint 非準拠
    alt 鍵読み込み失敗 (env 不整合 / PKCS#1 / encrypted)
        KS-->>Init: error
        Init-->>Init: logger.Error + os.Exit(1)
    end
    KS-->>Init: KeySet (staticKeySet, 1 鍵保持)
    Init->>Init: logger.Info "起動鍵情報" (kid / alg=RS256 / 取得経路 / CORE_ENV)
    alt CORE_OIDC_DEV_GENERATE_KEY=1
        Init->>Init: logger.Warn "dev 鍵生成モード = 単一 Pod 専用"
    end
    Init->>Srv: New(cfg, logger, pool, keystore) + ListenAndServe
    Srv->>Srv: chi router 構築<br/>middleware D1: request_id → access_log → recover<br/>routes:<br/> /health/* (M0.3)<br/> /.well-known/openid-configuration<br/> /jwks<br/> /authorize → notimpl.Handler("M1.2")<br/> /token → notimpl.Handler("M1.3")<br/> /userinfo → notimpl.Handler("M1.4")
```

### Discovery 取得フロー

```mermaid
sequenceDiagram
    autonumber
    participant RP as RP (go-oidc 等)
    participant LB as K8s Ingress (TLS 終端)
    participant MW as middleware (D1)
    participant H as discovery handler
    participant Cfg as config (issuer + endpoints)

    RP->>LB: GET https://id.example.com/.well-known/openid-configuration
    LB->>MW: GET /.well-known/openid-configuration
    MW->>MW: request_id 生成 (UUID v7) + ctx 注入
    MW->>MW: access_log "access" (method/path/status/duration_ms)<br/>※ defer で終了時出力
    MW->>H: ServeHTTP
    H->>Cfg: 構築済メタデータ取得 (起動時に固定)
    Cfg-->>H: Discovery JSON 構造体
    H->>H: 自前固定キー順 marshal (決定的、F-21)
    H->>H: ETag = sha256(body)[:16] base64url-no-padding
    H->>H: Cache-Control = CORE_OIDC_DISCOVERY_MAX_AGE で切替
    alt If-None-Match ヘッダ一致
        H-->>RP: 304 Not Modified (body なし、ETag 一致確認のみ)
    else 通常レスポンス
        H-->>RP: 200 OK<br/>Content-Type: application/json<br/>Cache-Control: ...<br/>ETag: "..."<br/>body = Discovery metadata
    end
```

### JWKS 取得フロー

```mermaid
sequenceDiagram
    autonumber
    participant RP as RP (go-oidc 等)
    participant MW as middleware (D1)
    participant H as jwks handler
    participant KS as keystore.KeySet

    RP->>MW: GET /jwks
    MW->>H: ServeHTTP (request_id ctx 注入済)
    H->>KS: Verifying(ctx) → 公開鍵リスト
    Note right of KS: M1.1 = staticKeySet で 1 鍵のみ<br/>M2.x rotation 時は新旧複数鍵を返す
    KS-->>H: []KeyPair (公開鍵 + kid + alg)
    H->>H: jwx の jwk.Set + AddKey で構築<br/>alg=RS256 / kty=RSA / use=sig 明示セット
    H->>H: json.Marshal(set) (jwx Marshaler、決定的キー順)<br/>※ private 成分 (d/p/q/dp/dq/qi) は出力しない
    H->>H: ETag = sha256(body)[:16] base64url-no-padding
    H->>H: Cache-Control = "public, max-age=<CORE_OIDC_JWKS_MAX_AGE>, must-revalidate" (既定 300)
    alt If-None-Match ヘッダ一致
        H-->>RP: 304 Not Modified
    else 通常レスポンス
        H-->>RP: 200 OK<br/>Content-Type: application/json<br/>Cache-Control: public, max-age=300, must-revalidate<br/>ETag: "..."<br/>body = {"keys":[{kty,use,alg,kid,n,e}]}
    end
```

### 未実装 endpoint (503 stub) フロー

```mermaid
sequenceDiagram
    autonumber
    participant RP as RP (go-oidc 等)
    participant MW as middleware (D1)
    participant H as notimpl.Handler(milestone)

    RP->>MW: GET /authorize (or POST /token, GET /userinfo)
    MW->>H: ServeHTTP (request_id ctx 注入済)
    H->>H: w.Header().Set("Content-Type","application/json")
    H->>H: w.Header().Set("Cache-Control","no-store")
    H->>H: w.WriteHeader(503)
    H->>H: json.NewEncoder(w).Encode(<br/> {"error":"endpoint_not_implemented",<br/>  "available_at":"M1.2"})
    Note right of H: milestone は endpoint ごと固定<br/>/authorize=M1.2 / /token=M1.3 / /userinfo=M1.4<br/>※ apperror.WriteJSON は使わない<br/>  (apperror は code/message/request_id 形式で<br/>   OIDC 標準と異なるため、論点 #8 確定)
    H-->>RP: 503 Service Unavailable + 機械可読 JSON
    Note over MW: access_log "access" status=503 (WARN)<br/>※ M1.2-1.4 で本実装に差し替え時は<br/>  server.go の route 登録 1 行を置き換え
```

### M2.x 鍵 rotation 予告 (本マイルストーン非実装、F-22 参照)

M2.x で zero-downtime rotation を実装する際の overlap window 設計予告 (規約書に明記、本マイルストーンでは実装しない)。

```mermaid
sequenceDiagram
    autonumber
    participant Op as 運用者
    participant K8s as K8s Secret
    participant KS as keystore.KeySet (M2.x)
    participant RP as RP (go-oidc 等)

    Note over Op,RP: 前提: M2.x で staticKeySet → multiKeySet に拡張<br/>Active(ctx) = 新鍵 / Verifying(ctx) = [新鍵, 旧鍵]
    Op->>K8s: 新鍵 (kid=NEW) を Secret に追記
    Op->>KS: rotation API 起動 (M2.x で別途設計)
    KS->>KS: Active を NEW に切替、Verifying は [NEW, OLD]
    Note right of KS: overlap window 開始<br/>>= max-age + RP 再取得遅延 + 時計ずれバッファ<br/>(JWKS max-age=300s なら実務目安 15-30 分以上)
    par 旧鍵で署名された ID Token を持つ RP
        RP->>KS: GET /jwks (キャッシュ TTL 期限切れ後)
        KS-->>RP: {keys:[NEW, OLD]} (両方含む)
        RP->>RP: kid=OLD で検証成功
    and 新鍵で署名された ID Token を持つ RP
        RP->>KS: GET /jwks
        KS-->>RP: {keys:[NEW, OLD]}
        RP->>RP: kid=NEW で検証成功
    end
    Note over Op,RP: overlap window 終了 (時間経過)
    Op->>K8s: 旧鍵 (OLD) を Secret から削除
    Op->>KS: keystore reload (Pod 全再起動 or hot reload、M2.x 設計次第)
    KS->>KS: Verifying = [NEW] のみ
    Note right of KS: M1.1 では本フローを実装しない、Pod 全停止 → 起動 (F-24)
```

> **本マイルストーン (M1.1) の鍵更新運用制約**: M1.1 で Secret 内の鍵を変更したい場合は **全 Pod 停止 → 新鍵で再起動** 必須 (F-24)。rolling restart 中は Pod 間で旧鍵 / 新鍵が混在し署名検証不整合となるため。zero-downtime rotation は M2.x で本格実装。

## テスト観点

`/spec-tests` フェーズで観点を網羅化。雛形:

### バックエンド単体

- Discovery handler: F-1〜F-4 のフィールド網羅、Cache-Control / ETag ヘッダ
- JWKS handler: F-5 のフィールド、F-6 ヘッダ、F-21 決定的シリアライズ、`alg=RS256` / `kty=RSA` / `use=sig` の明示セット、**private 成分 (`d`/`p`/`q`/`dp`/`dq`/`qi`) が JWKS レスポンスに含まれない** ことの確認テスト
- JWKS golden / 決定論性テスト (論点 #10 + Codex HIGH 1): (1) 同一鍵セットを 100 回 marshal して全て同一バイト列、(2) `go test -count=100` で安定性確認、(3) golden ファイル (`testdata/jwks_golden.json` 等) で固定バイト列を期待値とし、jwx バージョン更新時に差分が出たら golden 更新は自動禁止 (PR レビュー必須)
- ETag 契約テスト: (1) 固定の鍵セット → 既知の ETag 値を期待、(2) 鍵 1 つの場合は順序入替え自体不可だが M2.x で複数鍵に拡張時に順序入替えで ETag 不変を確認、(3) 鍵追加 / 削除でのみ ETag 変化
- keystore: PEM 読み込み (PKCS#8 正常系)、kid 自動算出 (F-11 決定論、RFC 7638 thumbprint 非準拠を意識)、Active / Verifying I/F
- keystore 鍵長透過 (論点 #16): RSA 1024 / 2048 / 3072 / 4096 bit を `crypto/rsa.GenerateKey` で動的生成 → PKCS#8 PEM 化 → keystore でロード → JWKS シリアライズまで通ることを確認 (アプリが任意 bit 数の RSA 鍵に透過であることを機械的に保証)
- keystore 異常系 (論点 #10 Codex MEDIUM 2): (a) PKCS#1 PEM (`-----BEGIN RSA PRIVATE KEY-----`) を渡したら明確エラー (`KEY_FORMAT_NOT_PKCS8` 等の内部 code 候補)、(b) encrypted PEM (パスフレーズ付き) は非対応、起動時エラー文言で運用回避策 (復号済み配置) を案内、(c) 1024 bit 等 RS256 として弱い鍵長は **拒否はしない** が WARN ログ
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

新設「OIDC OP 規約」節 (F-19、最低 6 項目 + 論点 #10 Codex 反映で 2 項目追加 = 計 8 項目):

1. issuer URL 規約 (https 必須 + dev 例外、末尾スラッシュ strip)
2. 環境変数一覧 (`CORE_OIDC_*` / `CORE_ENV`)
3. 鍵保管方式 (本番 = K8s Secret マウント / dev = devkeygen) + kid と fingerprint の関係 (F-11 / F-18 同値) + **鍵長ガイダンス (論点 #16)**: id-core は任意 bit 数の RSA 鍵を受け入れる (鍵長透過、env / config で強制しない)。推奨は **2048 bit 以上 (RS256 下限)、4096 bit まで実用範囲、>4096 bit は HTTP timeout 接近リスクあり**。本番鍵生成は id-core スコープ外 (運用側 = openssl / cloud KMS / HSM 等で生成)、devkeygen は dev 専用 (F-13)、本番ツール化しない。**kid は F-11 独自仕様 (公開鍵 DER SHA-256 先頭 24 hex) で RFC 7638 thumbprint 非準拠** — ログ / 運用文書では「kid」または「fingerprint」と呼び、「thumbprint」は使わない (論点 #10 Codex HIGH 2)
4. dev 鍵生成モードの単一 Pod 制約 (Helm/manifest 側で `replicas: 1` 強制、実 sample は M1.5)
5. 開発者向け運用手順 (`make dev-keygen` / docker compose / make run)
6. M1.1 の鍵更新運用制約 (F-24: Pod 全停止 → 起動) と M2.x の overlap window 予告 (F-22)
7. **鍵フォーマット受け入れポリシー (論点 #10 Codex MEDIUM 2)**: PKCS#8 のみ受け入れ。PKCS#1 (`-----BEGIN RSA PRIVATE KEY-----`) は明確エラーで起動失敗。encrypted PEM (パスフレーズ付き) は非対応、エラー文言で「復号済み PEM を K8s Secret に配置してください」と案内。1024 bit 以下は拒否しないが起動 WARN ログ
8. **JWKS 出力契約 (論点 #10 Codex HIGH 1 / LOW 2)**: jwx/v3 のバージョンは `go.mod` でマイナー / パッチまで実質固定 (Renovate / Dependabot で自動更新する場合も major / minor は手動レビュー必須)。`alg=RS256` / `kty=RSA` / `use=sig` を明示セット、private 成分 (`d`/`p`/`q`/`dp`/`dq`/`qi`) は JWKS に出力しない (golden / 契約テストで担保)

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

| フェーズ               | 状態   | 備考                                                                                                                                                                                                                                                                                                              |
| ---------------------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1. 起票                | 完了   | 2026-05-02、Issue #32 起点、要求 #32 確定情報引き継ぎ                                                                                                                                                                                                                                                             |
| 2. 下書き              | 完了   | 2026-05-02、`/spec-create` で雛形生成                                                                                                                                                                                                                                                                             |
| 3. 規約確認            | 完了   | 2026-05-03 完了。M0.1-0.3 既存実装を `/doc-review` 内で直接確認 (`core/cmd/core/main.go` 起動順序 / `core/internal/config/Config` 構造 / `core/internal/apperror/CodedError` (Kind 概念不在、SCREAMING_SNAKE_CASE 規約) / `core/internal/middleware/` D1 順序 / `core/Makefile` / `.gitignore`)。本書本文に反映済 |
| 4. 論点解決            | 完了   | 2026-05-03 完了。`/spec-resolve` で 14 件確定 (#1〜#14、#10/#11 は Codex セカンドオピニオン取得) + 論点 #16 新規追加 (本番鍵の責任分界) + #15 削除 (OpenAPI 未導入のため)                                                                                                                                         |
| 5. DB 設計             | 対象外 | 永続化なし                                                                                                                                                                                                                                                                                                        |
| 6. API 設計            | 雛形済 | エンドポイント / レスポンス / ヘッダ / env を記載済、論点解決後に追補                                                                                                                                                                                                                                             |
| 7. 認可設計            | 完了   | 公開エンドポイントのためマスター対象外、本書に既述                                                                                                                                                                                                                                                                |
| 8. 図                  | 完了   | 2026-05-03 完了。`/spec-diagrams` で起動シーケンス + Discovery 取得 + JWKS 取得 + 503 stub + M2.x rotation 予告の Mermaid シーケンス図を 5 件作成 (UI なしのためフローチャートは省略)                                                                                                                             |
| 9. テスト設計          | 完了   | 2026-05-03 完了 (論点解決時に拡充)。バックエンド単体 (Discovery/JWKS/keystore/notimpl/config) + golden / 決定論性 / ETag 契約 / 鍵長透過 / 異常系 (PKCS#1/encrypted/短鍵) / private 成分非出力 + ContractTest 5 ケース + 統合 (env 切替) + セキュリティ (redact / fail-fast) を網羅。data-testid は本スコープ外   |
| 10. 差分整理           | 完了   | 2026-05-03 完了 (論点解決時に拡充)。conventions / registry / patterns / testing / Makefile / .gitignore への差分を網羅 (F-19 規約 8 項目 + Codex 反映の鍵フォーマット受け入れ + JWKS 出力契約を含む)                                                                                                              |
| 11. 実装プロンプト生成 | 完了   | 2026-05-03 完了。`prompts/P1_01_core_config_keystore_devkeygen.md` / `P2_01_core_oidc_discovery.md` / `P3_01_core_oidc_jwks_notimpl.md` / `P4_01_core_main_integration_context.md` の 4 ファイル生成 (Codex CLI 想定、テンプレート 10 必須セクション網羅、自己完結型)                                             |

## 変更履歴

| 日付       | 変更内容                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| ---------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 2026-05-02 | 起票 (Issue #32 / 要求 #32 確定情報引き継ぎ)、`/spec-create` で雛形生成。要件解釈・実装対象・API 設計雛形・認可設計 (公開エンドポイントのため対象外)・テスト観点雛形・差分整理雛形を記載。論点 15 件を初期登録 (#1〜#15)                                                                                                                                                                                                                                                                                                                                                                 |
| 2026-05-03 | `/doc-review` セルフレビュー (HIGH=3 / MEDIUM=3 / LOW=1) を反映: (HIGH 1) `apperror.AppError` → `apperror.CodedError` に表記統一、(HIGH 2) 503 stub の `Retry-After` ヘッダ削除 (RFC 7231 違反、`Cache-Control: no-store` に置換)、(HIGH 3) 内部 code (SCREAMING_SNAKE_CASE) と OIDC レスポンス `error` フィールド (snake_case) の二重化を論点 #8 で明示、(MEDIUM 1) 論点 #9 を `kind` 不在踏まえてプレーン error 経路に修正、(MEDIUM 2) 論点 #15 (OpenAPI) 削除、(MEDIUM 3) 起動シーケンス記述を実装準拠に修正、(LOW 1) ETag 仕様明文化 (SHA-256 先頭 16 バイト → base64url-no-padding) |
| 2026-05-03 | 論点 #1 確定 (`/spec-resolve`): keystore パッケージ配置 = **A (`core/internal/keystore/` 単独配置)**。理由 = 将来 M4.x の上流 IdP RP 連携 / その他 HMAC 署名等での再利用余地、依存方向 (oidc → keystore の一方向) の素直さ                                                                                                                                                                                                                                                                                                                                                               |
