# P2_01: core/internal/oidc/discovery (Discovery handler + メタデータ + ContractTest)

M1.1 の Discovery メタデータ公開 endpoint (`GET /.well-known/openid-configuration`) を実装する。P1 (config + keystore) 完了後に着手、P3 (JWKS + notimpl) と並列実装可。

## 絶対ルール

- **コードベース探索禁止**: Grep, Glob, Read による既存コードの探索的な読み取りを行わない
- **Explore エージェント起動禁止**: サブエージェントによるコードベース調査を行わない
- 実装に必要な情報は全て、以下の context ファイルとこのプロンプト内に記載済み
- context に記載がなく実装判断できない場合は、探索ではなく**ユーザーに質問**すること
- 変更対象ファイルの直接の Read / Edit のみ許可 (探索目的の Read は禁止)
- **Codex レビューをスキップしない**: 各作業ステップに Codex レビューが含まれている。レビューを受けずに次のステップに進むことは禁止
- **完了条件のタスク化**: 作業開始前に「完了条件」セクションの各項目を TaskCreate で登録し、各ステップ完了時に TaskUpdate で状態を更新すること。タスク化せずに作業を開始することは禁止
- **`apperror.WriteJSON` は使わない** (本 endpoint のレスポンス形式は OIDC Discovery 1.0 + RFC 8414 の JSON 構造で、apperror の `code/message/request_id` 形式と異なる)
- **OIDC Discovery 1.0 / RFC 8414 仕様準拠**: 必須フィールド省略禁止、メディアタイプは `application/json` (MUST)
- **server.go への route 登録は本タスクで行わない** (P4 で集約)、本タスクは handler 関数とパッケージ単体のみ

## 作業ステップ (この順序で実行すること)

### ステップ 1: Discovery メタデータ構造体 + 構築ロジック

1. テスト先 (`core/internal/oidc/discovery/metadata_test.go`):
   - **基本ケース**: `CORE_OIDC_ISSUER=https://id.example.com` で各フィールドが期待値:
     - `issuer = https://id.example.com`
     - `authorization_endpoint = https://id.example.com/authorize`
     - `token_endpoint = https://id.example.com/token`
     - `userinfo_endpoint = https://id.example.com/userinfo`
     - `jwks_uri = https://id.example.com/jwks`
     - `response_types_supported = ["code"]`
     - `grant_types_supported = ["authorization_code"]`
     - `subject_types_supported = ["public"]`
     - `id_token_signing_alg_values_supported = ["RS256"]`
     - `scopes_supported = ["openid"]`
     - `token_endpoint_auth_methods_supported = ["client_secret_basic"]`
   - **subpath ケース**: `https://example.com/id-core` で `authorization_endpoint = https://example.com/id-core/authorize` 等 (Q8 / 論点 #6)
   - **末尾スラッシュ ケース**: `https://example.com/id-core/` 入力で `issuer = https://example.com/id-core` (config 側で strip 済前提) → endpoint も同じく strip 後 issuer 配下
   - **dev ケース**: `http://localhost:8080` で `authorization_endpoint = http://localhost:8080/authorize`
   - **非標準ポート ケース**: `https://id.example.com:9443` で `authorization_endpoint = https://id.example.com:9443/authorize`
   - **endpoint override ケース**: `CORE_OIDC_AUTHORIZATION_ENDPOINT=https://other.example.com/auth` で issuer 由来の構築値より優先 (論点 #12)
   - **jwks_uri override ケース**: `CORE_OIDC_JWKS_URI=https://cdn.example.com/jwks.json` で override 反映
2. 実装 (`core/internal/oidc/discovery/metadata.go`):

   ```go
   type Metadata struct {
       Issuer                            string   `json:"issuer"`
       AuthorizationEndpoint             string   `json:"authorization_endpoint"`
       TokenEndpoint                     string   `json:"token_endpoint"`
       UserinfoEndpoint                  string   `json:"userinfo_endpoint"`
       JWKSURI                           string   `json:"jwks_uri"`
       ResponseTypesSupported            []string `json:"response_types_supported"`
       GrantTypesSupported               []string `json:"grant_types_supported"`
       SubjectTypesSupported             []string `json:"subject_types_supported"`
       IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
       ScopesSupported                   []string `json:"scopes_supported"`
       TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
   }

   func Build(cfg config.OIDCConfig) Metadata { ... }
   ```

   - 各 endpoint URL は `url.URL.JoinPath` で組み立て、override 設定があればそちらを優先

3. `lint` & `test` パス
4. **Codex レビュー実行**
5. 指摘対応 → 次のステップ

### ステップ 2: 決定的シリアライズ + ETag 計算

1. テスト先 (`core/internal/oidc/discovery/serialize_test.go`):
   - 同一 `Metadata` を 100 回 marshal して全て同一バイト列 (F-21 決定的シリアライズ)
   - キー順序が JSON tag の宣言順と一致 (struct field 順 = OIDC Discovery 1.0 慣習順)
   - ETag = `"<base64url-no-pad of sha256(body)[0:16]>"` (24 文字、引用符込み)、論点 #4 確定
   - 同一 body から常に同じ ETag (100 回呼び出し全て一致)
2. 実装 (`core/internal/oidc/discovery/serialize.go`):
   - `Marshal(m Metadata) ([]byte, error)`: 標準 `encoding/json` の Marshal はキー順序が struct field 順なのでそのまま使う (`json.Marshal(m)`)、ただし indent / 空白なし
   - `ETag(body []byte) string`: `crypto/sha256` で 32B → 先頭 16B → `base64.RawURLEncoding.EncodeToString` → ダブルクォートで囲む
3. `lint` & `test` パス
4. **Codex レビュー実行**
5. 指摘対応 → 次のステップ

### ステップ 3: Discovery handler + ContractTest 5 ケース

1. テスト先 (`core/internal/oidc/discovery/handler_test.go`):
   - **正常系 200**: `httptest.NewRecorder` + GET → `Content-Type: application/json` + `ETag` ヘッダあり + `Cache-Control` ヘッダあり + body が想定 JSON 構造
   - **Cache-Control 切替**:
     - `CORE_OIDC_DISCOVERY_MAX_AGE=0` (既定) → `Cache-Control: no-cache, must-revalidate`
     - `CORE_OIDC_DISCOVERY_MAX_AGE=600` → `Cache-Control: public, max-age=600, must-revalidate`
   - **`If-None-Match` 一致 304**: 1 回目 GET → ETag を取得 → 2 回目 GET (If-None-Match ヘッダ付き) → 304 Not Modified、body なし
   - **`If-None-Match` 不一致 200**: 別の ETag を渡したら 200 で body 返却
   - **ContractTest 5 ケース** (Q8、F-17 確定): `metadata_test.go` のテーブル駆動と handler 経由の両方で、同じ 5 ケース (標準 / subpath / 末尾スラッシュ / dev / 非標準ポート) を網羅
2. 実装 (`core/internal/oidc/discovery/handler.go`):

   ```go
   type Handler struct {
       metadata Metadata
       body     []byte // 起動時に 1 回 marshal してキャッシュ (F-21 + パフォーマンス)
       etag     string // 起動時に 1 回計算してキャッシュ
       cache    string // 起動時に Cache-Control ヘッダ値を確定
   }

   func New(cfg config.OIDCConfig) (*Handler, error) {
       m := Build(cfg)
       body, err := Marshal(m)
       if err != nil { return nil, err }
       return &Handler{
           metadata: m,
           body:     body,
           etag:     ETag(body),
           cache:    cacheControlValue(cfg.DiscoveryMaxAge),
       }, nil
   }

   func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
       if r.Header.Get("If-None-Match") == h.etag {
           w.WriteHeader(http.StatusNotModified)
           return
       }
       w.Header().Set("Content-Type", "application/json")
       w.Header().Set("Cache-Control", h.cache)
       w.Header().Set("ETag", h.etag)
       w.WriteHeader(http.StatusOK)
       w.Write(h.body)
   }

   func cacheControlValue(maxAge int) string {
       if maxAge <= 0 { return "no-cache, must-revalidate" }
       return fmt.Sprintf("public, max-age=%d, must-revalidate", maxAge)
   }
   ```

3. `lint` & `test` パス
4. **Codex レビュー実行**
5. 指摘対応 → 次のステップ

### ステップ最終: 全体テスト + カバレッジ確認

1. `make -C core lint test` 緑
2. `make -C core test-cover` でカバレッジ確認:
   - discovery: 95% 以上 (handler + metadata + serialize 全件)
3. 完了報告 (`docs/context/` 更新は P4 でまとめて行うため本タスクではスキップ)

## 実装コンテキスト

以下のファイルを読み取ってから実装を開始すること:

```
CONTEXT_DIR="docs/context"
```

- `${CONTEXT_DIR}/app/architecture.md` (全体構成)
- `${CONTEXT_DIR}/backend/conventions.md` (エラー / ログ / 環境変数命名)
- `${CONTEXT_DIR}/backend/patterns.md` (handler 配置 / chi router 統合パターン)
- `${CONTEXT_DIR}/backend/registry.md` (パッケージ一覧)
- `${CONTEXT_DIR}/testing/backend.md` (テスト規約)

設計書: `docs/specs/32/index.md` (特に「API 設計」「Discovery レスポンスヘッダ」「論点 #4, #6, #7, #12」「フロー図 / シーケンス図 → Discovery 取得フロー」)

要求文書: `docs/requirements/32/index.md` (F-1, F-2, F-3, F-4, F-15, F-16, F-17, F-21)

適用範囲: `core/internal/oidc/discovery/` のみ。`server.go` への route 登録は P4 で行う

## 前提条件

- **P1 完了** (`core/internal/config/OIDCConfig` + `core/internal/keystore/` がマージ済)
- 本タスクは P4 (main 統合 + server.go route 登録) の前提
- P3 (JWKS + notimpl) と並列実装可能 (依存パッケージなし)

## 不明点ハンドリング

- 矛盾・欠落・未定義がある場合は作業を止める
- 勝手な推測で実装を進めない
- 質問時は: 止まっている作業単位 / 判断が必要な論点 / 選択肢を整理して提示
- 例: 「Cache-Control の `no-cache` と `max-age=0` の挙動差」「OIDC Discovery 1.0 で `userinfo_endpoint` が技術的に OPTIONAL だが M1.1 では広告するか」等で迷ったらユーザー確認

## タスク境界

### 実装する範囲

- `core/internal/oidc/discovery/metadata.go` + テスト (Metadata 構造体 + Build 関数)
- `core/internal/oidc/discovery/serialize.go` + テスト (Marshal + ETag)
- `core/internal/oidc/discovery/handler.go` + テスト (HTTP handler + Cache-Control + If-None-Match 304 応答)
- ContractTest 5 ケース (Q8 / F-17) を `metadata_test.go` と `handler_test.go` の両方で網羅

### 実装しない範囲 (後続タスク)

- JWKS handler (P3)
- 503 stub (notimpl) handler (P3)
- `core/internal/server/server.go` への route 登録 + middleware チェーン適用 (P4)
- main.go の起動シーケンス統合 (P4)
- `docs/context/` 更新 (P4)

## 設計仕様 (設計書から本タスク該当箇所を抜粋)

### Discovery メタデータ仕様 (F-2, F-3, F-4)

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

### Discovery レスポンスヘッダ (F-1, Q15, 論点 #4)

| ヘッダ          | 値                                                                                                            |
| --------------- | ------------------------------------------------------------------------------------------------------------- |
| `Content-Type`  | `application/json` (RFC 8414 §3 MUST)                                                                         |
| `Cache-Control` | `CORE_OIDC_DISCOVERY_MAX_AGE=0` → `no-cache, must-revalidate` / `>0` → `public, max-age=<N>, must-revalidate` |
| `ETag`          | strong ETag (例: `"abcd_efghIJKLmnopQRSt"`、24 文字、引用符込み)                                              |

### ETag 計算 (論点 #4 確定)

```go
import (
    "crypto/sha256"
    "encoding/base64"
)

func ETag(body []byte) string {
    sum := sha256.Sum256(body)
    return `"` + base64.RawURLEncoding.EncodeToString(sum[:16]) + `"`
}
```

### subpath / endpoint override 仕様 (論点 #6, #12)

- subpath issuer (`https://example.com/id-core`): 末尾スラッシュ無で扱い、各 endpoint は `url.URL.JoinPath` で `/authorize`, `/token`, `/userinfo`, `/jwks` を結合
- endpoint override (`CORE_OIDC_<ENDPOINT>_ENDPOINT`) があれば issuer 由来の構築値より優先
- `CORE_OIDC_JWKS_URI` も同様に override 可能

### ContractTest 5 ケース (Q8, F-17)

| #   | issuer                                                | 期待 endpoint (例: authorize)           |
| --- | ----------------------------------------------------- | --------------------------------------- |
| 1   | `https://id.example.com`                              | `https://id.example.com/authorize`      |
| 2   | `https://example.com/id-core`                         | `https://example.com/id-core/authorize` |
| 3   | `https://example.com/id-core/` (config 側で strip 済) | `https://example.com/id-core/authorize` |
| 4   | `http://localhost:8080`                               | `http://localhost:8080/authorize`       |
| 5   | `https://id.example.com:9443`                         | `https://id.example.com:9443/authorize` |

## テスト観点 (本タスク該当のみ)

### discovery 単体

- **メタデータ構築** (`Build`):
  - 全 11 フィールドが期待値で埋まる
  - ContractTest 5 ケース (issuer 形式 5 種) でテーブル駆動
  - endpoint 個別 override が反映される (authorize / token / userinfo / jwks_uri、独立に override 可能)
- **シリアライズ** (`Marshal`):
  - 100 回呼び出して全て同一バイト列 (F-21)
  - JSON のキー順序は struct field 順 (人間可読性 + 仕様慣習)
- **ETag**:
  - 同一 body から同じ ETag (100 回)
  - body が 1 バイト変わったら ETag が変わる
  - 形式 = strong ETag (ダブルクォート囲み)、24 文字
- **handler**:
  - 200 + Content-Type / Cache-Control / ETag ヘッダ + body
  - `Cache-Control` 切替 (max-age=0 vs >0)
  - `If-None-Match` 一致時 304 (body 空)
  - `If-None-Match` 不一致時 200 (body 返却)
  - GET 以外 (POST 等) の挙動は本タスク非対象 (chi router 側が 405 を返す、P4 で確認)

## Codex レビューコマンド (各ステップで使用)

```bash
codex exec --full-auto -c model_reasoning_effort="medium" \
  "まず以下の context ファイルを読み取れ:
   - docs/context/app/architecture.md
   - docs/context/backend/conventions.md
   - docs/context/backend/patterns.md
   - docs/context/backend/registry.md
   - docs/context/testing/backend.md
   設計書: docs/specs/32/index.md (特に API 設計 / 論点 #4, #6, #7, #12 / フロー図)
   要求文書: docs/requirements/32/index.md (F-1, F-2, F-3, F-4, F-17, F-21)

   その上で git diff をレビューせよ。

   Check (本タスクは Discovery handler + metadata + serialize):
   1) TDD compliance (テスト先行 / カバレッジ 95%+)
   2) OIDC Discovery 1.0 / RFC 8414 仕様準拠 (必須フィールド網羅、メディアタイプ application/json)
   3) ContractTest 5 ケース (標準 / subpath / 末尾スラッシュ / dev / 非標準ポート) の網羅
   4) endpoint override (authorize/token/userinfo/jwks_uri 個別) の優先順位
   5) 決定的シリアライズ (100 回 marshal で同一バイト列、F-21)
   6) ETag 仕様準拠 (sha256 → 先頭 16B → base64url-no-padding → strong ETag、24 文字、引用符込み)
   7) Cache-Control 切替の正しさ (max-age=0 → no-cache / >0 → public, max-age, must-revalidate)
   8) If-None-Match 304 応答の実装 (body なし、ETag 比較は文字列完全一致)
   9) handler 構築の起動時キャッシュ (body / etag / cache を起動時に 1 回計算してフィールド保持)
   10) apperror.WriteJSON 不使用、OIDC 標準準拠の生 JSON 書き込み
   11) 探索禁止違反がないか

   Report issues as CRITICAL/HIGH/MEDIUM/LOW in Japanese. Last section must be ## Summary with counts and gate verdict."
```

## 完了条件

- [ ] ステップ 1: Metadata 構造体 + Build 関数 + ContractTest 5 ケース 完了 + Codex ゲート PASS
- [ ] ステップ 2: 決定的シリアライズ (Marshal) + ETag 計算 完了 + Codex ゲート PASS
- [ ] ステップ 3: Discovery handler + Cache-Control 切替 + If-None-Match 304 完了 + Codex ゲート PASS
- [ ] `make -C core lint test` 緑、discovery カバレッジ 95%+
- [ ] PR 作成 (`/pr-codex-review {番号}` で最終 Codex レビュー、CRITICAL=0/HIGH=0/MEDIUM<3 で main にマージ)
- [ ] `docs/context/` 更新は本タスクではスキップ (P4 で集約)
- [ ] 未解決の仕様質問が残っていない
