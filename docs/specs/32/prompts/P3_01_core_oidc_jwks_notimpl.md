# P3_01: core/internal/oidc/jwks (JWKS handler) + core/internal/oidc/notimpl (503 stub)

M1.1 の JWKS endpoint (`GET /jwks`) と未実装 endpoint stub (`/authorize`, `/token`, `/userinfo`) を実装する。P1 (config + keystore) 完了後に着手、P2 (Discovery) と並列実装可。

## 絶対ルール

- **コードベース探索禁止**: Grep, Glob, Read による既存コードの探索的な読み取りを行わない
- **Explore エージェント起動禁止**: サブエージェントによるコードベース調査を行わない
- 実装に必要な情報は全て、以下の context ファイルとこのプロンプト内に記載済み
- context に記載がなく実装判断できない場合は、探索ではなく**ユーザーに質問**すること
- 変更対象ファイルの直接の Read / Edit のみ許可 (探索目的の Read は禁止)
- **Codex レビューをスキップしない**: 各作業ステップに Codex レビューが含まれている。レビューを受けずに次のステップに進むことは禁止
- **完了条件のタスク化**: 作業開始前に「完了条件」セクションの各項目を TaskCreate で登録し、各ステップ完了時に TaskUpdate で状態を更新すること。タスク化せずに作業を開始することは禁止
- **`apperror.WriteJSON` は使わない** (JWKS は OIDC / JWK 標準形式、notimpl は OIDC 標準 `error` フィールド)
- **`go.mod` への新規依存追加 (`lestrrat-go/jwx/v3`) はユーザー事前承認済とする** が、major / minor 変更時は再承認必要 (`.rulesync/rules/pr-review-policy.md` §3)
- **JWKS の private 成分 (`d`/`p`/`q`/`dp`/`dq`/`qi`) は絶対に出力しない** (Codex LOW 2 / 論点 #10)
- **server.go への route 登録は本タスクで行わない** (P4 で集約)、本タスクは handler パッケージ単体のみ

## 作業ステップ (この順序で実行すること)

### ステップ 1: jwx/v3 依存追加 + 公開鍵 → JWK 変換

1. `go get github.com/lestrrat-go/jwx/v3@latest` (`core/` ディレクトリ内で)、`go.mod` / `go.sum` を更新
2. テスト先 (`core/internal/oidc/jwks/jwk_test.go`):
   - `*rsa.PublicKey` → `jwk.Key` 変換 (`jwk.Import`)
   - `alg=RS256` / `kty=RSA` / `use=sig` / `kid` が明示セットされる
   - 同一公開鍵から繰り返し変換しても結果が等価
3. 実装 (`core/internal/oidc/jwks/jwk.go`):

   ```go
   func ToJWK(kp *keystore.KeyPair) (jwk.Key, error) {
       key, err := jwk.Import(kp.PublicKey)
       if err != nil { return nil, fmt.Errorf("jwk.Import: %w", err) }
       _ = key.Set(jwk.KeyTypeKey, jwa.RSA())
       _ = key.Set(jwk.AlgorithmKey, jwa.RS256())
       _ = key.Set(jwk.KeyUsageKey, "sig")
       _ = key.Set(jwk.KeyIDKey, kp.Kid)
       return key, nil
   }
   ```

4. `lint` & `test` パス
5. **Codex レビュー実行**
6. 指摘対応 → 次のステップ

### ステップ 2: JWK Set 構築 + 決定的シリアライズ + ETag

1. テスト先 (`core/internal/oidc/jwks/serialize_test.go`):
   - `jwk.Set` 構築 → `json.Marshal(set)` で 100 回呼び出して全て同一バイト列 (F-21、論点 #10 Codex HIGH 1)
   - **golden ファイルテスト**: 既知の鍵 (テスト内固定の RSA 鍵) でシリアライズした結果を `testdata/jwks_golden.json` に保存し、毎回比較
   - **private 成分非出力**: シリアライズ結果の JSON をパースして `d`, `p`, `q`, `dp`, `dq`, `qi` フィールドが含まれないことを確認 (Codex LOW 2)
   - ETag = `"<base64url-no-pad of sha256(body)[0:16]>"` (24 文字、引用符込み、論点 #4)
   - `go test -count=100` で安定性確認 (Codex テスト追加提案)
2. 実装 (`core/internal/oidc/jwks/serialize.go`):

   ```go
   func BuildSet(keys []*keystore.KeyPair) (jwk.Set, error) {
       set := jwk.NewSet()
       for _, kp := range keys {
           k, err := ToJWK(kp)
           if err != nil { return nil, err }
           if err := set.AddKey(k); err != nil { return nil, err }
       }
       return set, nil
   }

   func Marshal(set jwk.Set) ([]byte, error) {
       return json.Marshal(set)
   }

   func ETag(body []byte) string {
       sum := sha256.Sum256(body)
       return `"` + base64.RawURLEncoding.EncodeToString(sum[:16]) + `"`
   }
   ```

3. `lint` & `test` パス
4. **Codex レビュー実行**
5. 指摘対応 → 次のステップ

### ステップ 3: JWKS handler + Cache-Control + If-None-Match 304

1. テスト先 (`core/internal/oidc/jwks/handler_test.go`):
   - 200 + `Content-Type: application/json` (β 確定、論点 #7) + `Cache-Control: public, max-age=300, must-revalidate` (既定) + `ETag` ヘッダ + body
   - `CORE_OIDC_JWKS_MAX_AGE=600` で `Cache-Control: public, max-age=600, must-revalidate`
   - `CORE_OIDC_JWKS_MAX_AGE=0` で `Cache-Control: no-cache, must-revalidate`
   - `If-None-Match` 一致 304
   - `If-None-Match` 不一致 200
   - body は `{"keys":[{"kty":"RSA","use":"sig","alg":"RS256","kid":"...","n":"...","e":"..."}]}` 形式
2. 実装 (`core/internal/oidc/jwks/handler.go`):

   ```go
   type Handler struct {
       body  []byte
       etag  string
       cache string
   }

   func New(ks keystore.KeySet, maxAge int) (*Handler, error) {
       keys, err := ks.Verifying(context.Background())
       if err != nil { return nil, fmt.Errorf("keystore.Verifying: %w", err) }
       set, err := BuildSet(keys)
       if err != nil { return nil, err }
       body, err := Marshal(set)
       if err != nil { return nil, err }
       return &Handler{
           body:  body,
           etag:  ETag(body),
           cache: cacheControlValue(maxAge),
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

### ステップ 4: notimpl 503 stub handler

1. テスト先 (`core/internal/oidc/notimpl/handler_test.go`):
   - `Handler("M1.2")` で GET → 503 + `Content-Type: application/json` + `Cache-Control: no-store` + body = `{"error":"endpoint_not_implemented","available_at":"M1.2"}`
   - `Handler("M1.3")` で `available_at: M1.3`
   - `Handler("M1.4")` で `available_at: M1.4`
   - **`Retry-After` ヘッダが付かない** こと (RFC 7231 違反回避、論点 doc-review HIGH 2)
   - body の `error` フィールドは snake_case (`endpoint_not_implemented`、論点 #8 二重化方針: 内部 code は `ENDPOINT_NOT_IMPLEMENTED` 別管理)
   - GET 以外 (POST 等) も同じ 503 を返す (notimpl は method 不問、本実装が来たら method ごとに route 登録される)
2. 実装 (`core/internal/oidc/notimpl/handler.go`):

   ```go
   package notimpl

   func Handler(milestone string) http.HandlerFunc {
       return func(w http.ResponseWriter, r *http.Request) {
           w.Header().Set("Content-Type", "application/json")
           w.Header().Set("Cache-Control", "no-store")
           w.WriteHeader(http.StatusServiceUnavailable)
           _ = json.NewEncoder(w).Encode(map[string]string{
               "error":        "endpoint_not_implemented",
               "available_at": milestone,
           })
       }
   }
   ```

   - `apperror.WriteJSON` は使わない (apperror は `code/message/request_id` 形式で OIDC 標準と異なるため、論点 #8)
   - 内部 code (`ENDPOINT_NOT_IMPLEMENTED`、SCREAMING_SNAKE_CASE) は本タスクでは導入しない (apperror.CodedError への code 追加は P4 で apperror パッケージ側に集約) — もし本タスクで参照する必要があれば文字列リテラルで使う

3. `lint` & `test` パス
4. **Codex レビュー実行**
5. 指摘対応 → 次のステップ

### ステップ最終: 全体テスト + カバレッジ確認

1. `make -C core lint test` 緑
2. `make -C core test-cover` でカバレッジ確認:
   - jwks: 95% 以上 (handler + serialize + jwk + golden 全件)
   - notimpl: 100% (handler のみ、シンプル)
3. `go test ./internal/oidc/jwks -count=100` で決定論性安定確認 (Codex 提案)
4. 完了報告 (`docs/context/` 更新は P4 でまとめて行うため本タスクではスキップ)

## 実装コンテキスト

以下のファイルを読み取ってから実装を開始すること:

```
CONTEXT_DIR="docs/context"
```

- `${CONTEXT_DIR}/app/architecture.md`
- `${CONTEXT_DIR}/backend/conventions.md`
- `${CONTEXT_DIR}/backend/patterns.md`
- `${CONTEXT_DIR}/backend/registry.md`
- `${CONTEXT_DIR}/testing/backend.md`

設計書: `docs/specs/32/index.md` (特に「JWKS レスポンスヘッダ」「未実装 endpoint レスポンス (503)」「論点 #4, #7, #8, #10」「フロー図 / シーケンス図 → JWKS 取得 / 503 stub」)

要求文書: `docs/requirements/32/index.md` (F-5, F-6, F-15, F-16, F-21, F-23)

適用範囲: `core/internal/oidc/jwks/` + `core/internal/oidc/notimpl/`。`server.go` への route 登録は P4 で行う

## 前提条件

- **P1 完了** (`core/internal/keystore/` がマージ済、`KeySet` I/F 利用可能)
- 本タスクは P4 (main 統合 + server.go route 登録) の前提
- P2 (Discovery handler) と並列実装可能 (依存パッケージなし)
- 新規依存 `github.com/lestrrat-go/jwx/v3` の追加はユーザー事前承認済 (Q3 確定)

## 不明点ハンドリング

- 矛盾・欠落・未定義がある場合は作業を止める
- 勝手な推測で実装を進めない
- 質問時は: 止まっている作業単位 / 判断が必要な論点 / 選択肢を整理して提示
- 例: 「jwx/v3 の Set Marshal がキー順序を保証するか不安、自前 marshal に切替えるか」「golden ファイルのバイト列が jwx の minor バージョンアップで変わったときの対処」等で迷ったらユーザー確認

## タスク境界

### 実装する範囲

- `core/internal/oidc/jwks/jwk.go` + テスト (公開鍵 → JWK 変換、`alg`/`kty`/`use`/`kid` 明示セット)
- `core/internal/oidc/jwks/serialize.go` + テスト (`BuildSet` + `Marshal` + `ETag`、決定論性 + private 成分非出力 + golden)
- `core/internal/oidc/jwks/handler.go` + テスト (HTTP handler + Cache-Control + If-None-Match 304)
- `core/internal/oidc/notimpl/handler.go` + テスト (503 stub、`Handler(milestone string)` ファクトリ)
- `core/go.mod` / `core/go.sum` への `lestrrat-go/jwx/v3` 追加
- `core/internal/oidc/jwks/testdata/jwks_golden.json` (テスト用固定鍵 + 期待 JSON バイト列)

### 実装しない範囲 (後続タスク)

- `core/internal/server/server.go` への route 登録 (P4)
- main.go の起動シーケンス統合 (P4)
- `apperror.CodedError` への `ENDPOINT_NOT_IMPLEMENTED` 内部 code 追加 (P4 で apperror パッケージに集約)
- `docs/context/` 更新 (P4)
- M2.x の複数鍵対応 (rotation、本マイルストーン非実装)

## 設計仕様 (設計書から本タスク該当箇所を抜粋)

### JWKS レスポンス (F-5, F-6)

レスポンス body 例 (1 鍵 RSA 2048):

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

| ヘッダ          | 値                                                                                                                       |
| --------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `Content-Type`  | `application/json` (大手準拠 = Google/Microsoft/Okta、論点 #7 で β 確定)                                                 |
| `Cache-Control` | `public, max-age=<X>, must-revalidate` (X = `CORE_OIDC_JWKS_MAX_AGE` 既定 300、`0` 指定時は `no-cache, must-revalidate`) |
| `ETag`          | strong ETag (JWKS バイト列由来、決定的、論点 #4)                                                                         |

### 未実装 endpoint レスポンス (F-23、503 stub)

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

`available_at` は endpoint ごとに固定 (`/authorize`=M1.2 / `/token`=M1.3 / `/userinfo`=M1.4)。**`Retry-After` ヘッダは付与しない** (RFC 7231 §7.1.3 で値は HTTP-date / delta-seconds のみ許容、`M1.2` のような文字列は規約違反)。

### jwx/v3 採用範囲 (論点 #10 確定)

- a. 公開鍵 → JWK 変換 = `jwk.Import` (jwx)
- b. JWK Set 構築 = `jwk.NewSet()` + `set.AddKey()` (jwx)
- c. JWK Set marshal = `json.Marshal(set)` (jwx Marshaler、決定的キー順)
- d. kid 算出 = **自前** (P1 keystore で実装済、F-11)
- e. PEM PKCS#8 ロード = `crypto/x509.ParsePKCS8PrivateKey` (P1 keystore で実装済)

### Codex セカンドオピニオン反映 (論点 #10)

- jwx バージョン固定 (`go.mod` で実質固定、Renovate / Dependabot は major/minor 自動更新を抑止)
- golden / 契約テスト必須化
- ETag 安定性テスト (`go test -count=100`)
- `alg=RS256` / `kty=RSA` / `use=sig` を明示セット
- private 成分 (`d`/`p`/`q`/`dp`/`dq`/`qi`) が JWKS に出力されないテスト

## テスト観点 (本タスク該当のみ)

### jwks 単体

- **JWK 変換** (`ToJWK`):
  - 公開鍵 → JWK で `kty=RSA` / `use=sig` / `alg=RS256` / `kid` 明示セット
  - 同一入力で 100 回呼び出して結果が等価
- **JWK Set + 決定的シリアライズ** (`BuildSet` + `Marshal`):
  - 100 回 marshal で同一バイト列 (F-21)
  - golden ファイル (`testdata/jwks_golden.json`) との完全一致
  - private 成分非出力 (`d`, `p`, `q`, `dp`, `dq`, `qi` が JSON に含まれない)
  - 1 鍵 / 2 鍵 (M2.x 想定の forward-compat テスト) で順序が安定
- **ETag**:
  - 同一 body から同じ ETag (100 回)
  - 形式 = strong ETag (24 文字、引用符込み)
- **handler**:
  - 200 + headers + body
  - Cache-Control 切替 (`max-age=300` 既定、`max-age=0` で `no-cache`)
  - `If-None-Match` 304
- **`go test -count=100`** で全テスト 100 回繰り返しても安定 (決定論性確認)

### notimpl 単体

- `Handler("M1.2")` → 503 + `Cache-Control: no-store` + body `{"error":"endpoint_not_implemented","available_at":"M1.2"}`
- `Handler("M1.3")` / `Handler("M1.4")` で `available_at` が変わる
- `Retry-After` ヘッダが**付かない**こと
- GET / POST / PUT / DELETE すべて同じ 503 (method 不問)
- `Content-Type: application/json` 固定

## Codex レビューコマンド (各ステップで使用)

```bash
codex exec --full-auto -c model_reasoning_effort="medium" \
  "まず以下の context ファイルを読み取れ:
   - docs/context/app/architecture.md
   - docs/context/backend/conventions.md
   - docs/context/backend/patterns.md
   - docs/context/backend/registry.md
   - docs/context/testing/backend.md
   設計書: docs/specs/32/index.md (特に JWKS / 503 stub セクション、論点 #4, #7, #8, #10)
   要求文書: docs/requirements/32/index.md (F-5, F-6, F-21, F-23)

   その上で git diff をレビューせよ。

   Check (本タスクは JWKS handler + notimpl 503 stub):
   1) TDD compliance (テスト先行 / カバレッジ jwks 95%+ / notimpl 100%)
   2) JWK 仕様準拠 (RFC 7517、kty/use/alg/kid 明示セット)
   3) 決定的シリアライズ (100 回 marshal で同一バイト列、golden ファイル一致、F-21)
   4) private 成分 (d/p/q/dp/dq/qi) が JWKS に出力されない (Codex LOW 2)
   5) ETag 仕様準拠 (sha256 → 先頭 16B → base64url-no-padding → strong ETag)
   6) Content-Type = application/json (β 確定、論点 #7)
   7) Cache-Control 切替の正しさ (max-age=300 既定 / 0 で no-cache)
   8) If-None-Match 304 応答
   9) notimpl: Retry-After ヘッダ不在 (RFC 7231 違反回避)
   10) notimpl: Cache-Control: no-store
   11) notimpl: body の error フィールドが snake_case (endpoint_not_implemented)、apperror.WriteJSON 不使用
   12) notimpl: available_at が milestone ごと固定値で正しく出力 (M1.2/M1.3/M1.4)
   13) jwx バージョン固定 (go.mod / go.sum に lestrrat-go/jwx/v3 追加、major/minor 変更なし)
   14) 探索禁止違反がないか

   Report issues as CRITICAL/HIGH/MEDIUM/LOW in Japanese. Last section must be ## Summary with counts and gate verdict."
```

## 完了条件

- [ ] ステップ 1: jwx 依存追加 + 公開鍵 → JWK 変換 完了 + Codex ゲート PASS
- [ ] ステップ 2: JWK Set 構築 + 決定的シリアライズ + golden + ETag 完了 + Codex ゲート PASS
- [ ] ステップ 3: JWKS handler + Cache-Control + If-None-Match 304 完了 + Codex ゲート PASS
- [ ] ステップ 4: notimpl 503 stub handler 完了 + Codex ゲート PASS
- [ ] `make -C core lint test` 緑、jwks カバレッジ 95%+ / notimpl 100%
- [ ] `go test ./internal/oidc/jwks -count=100` で 100 回連続成功 (決定論性安定確認)
- [ ] PR 作成 (`/pr-codex-review {番号}` で最終 Codex レビュー、CRITICAL=0/HIGH=0/MEDIUM<3 で main にマージ)
- [ ] `docs/context/` 更新は本タスクではスキップ (P4 で集約)
- [ ] 未解決の仕様質問が残っていない
