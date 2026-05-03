# P1_01: core/ config 拡張 + keystore + devkeygen CLI

M1.1 (OIDC Discovery + JWKS) の **鍵管理基盤** を実装する。後続 P2 (Discovery) / P3 (JWKS + notimpl) / P4 (main 統合) の前提となる。

## 絶対ルール

- **コードベース探索禁止**: Grep, Glob, Read による既存コードの探索的な読み取りを行わない
- **Explore エージェント起動禁止**: サブエージェントによるコードベース調査を行わない
- 実装に必要な情報は全て、以下の context ファイルとこのプロンプト内に記載済み
- context に記載がなく実装判断できない場合は、探索ではなく**ユーザーに質問**すること
- 変更対象ファイルの直接の Read / Edit のみ許可 (探索目的の Read は禁止)
- **Codex レビューをスキップしない**: 各作業ステップに Codex レビューが含まれている。レビューを受けずに次のステップに進むことは禁止
- **完了条件のタスク化**: 作業開始前に「完了条件」セクションの各項目を TaskCreate で登録し、各ステップ完了時に TaskUpdate で状態を更新すること。タスク化せずに作業を開始することは禁止
- **UUID v4 禁止** (`uuid.New` / `uuid.NewRandom` 不可、本タスクで UUID は使用しないが原則として M0.x 規約踏襲)
- **`log.Fatal*` 禁止** (`logger.Error` + `os.Exit(1)`、Makefile lint で自動検査)
- **dev 鍵 (秘密鍵 / 公開鍵とも) を一切リポジトリにコミットしない** (F-14、`core/dev-keys/` を `.gitignore` に追加)

## 作業ステップ (この順序で実行すること)

### ステップ 1: config パッケージ拡張 (`CORE_ENV` strict + `CORE_OIDC_*`)

1. テスト先 (`core/internal/config/config_test.go` に追加):
   - `CORE_ENV` strict 検証: `prod` / `staging` / `dev` のみ受け入れ、空 / unset / `production` 等は起動失敗
   - `CORE_OIDC_ISSUER` 必須、`prod`/`staging` で `https://` 必須、`dev` で `http://` 許可、末尾スラッシュ strip
   - `CORE_OIDC_KEY_FILE` は `prod` で必須、`staging`/`dev` は `CORE_OIDC_DEV_GENERATE_KEY=1` で代替可
   - `CORE_OIDC_DEV_GENERATE_KEY=1` + `CORE_ENV=prod` → 起動失敗
   - `CORE_OIDC_KEY_ID` 任意 (空なら自動算出フラグを残す)
   - `CORE_OIDC_JWKS_MAX_AGE` 既定 300、範囲 0〜86400、範囲外は起動失敗
   - `CORE_OIDC_DISCOVERY_MAX_AGE` 既定 0、範囲 0〜86400、範囲外は起動失敗
   - `CORE_OIDC_AUTHORIZATION_ENDPOINT` / `CORE_OIDC_TOKEN_ENDPOINT` / `CORE_OIDC_USERINFO_ENDPOINT` / `CORE_OIDC_JWKS_URI` は任意 (未設定なら issuer から構築、設定があれば優先)
2. `core/internal/config/config.go` に `OIDCConfig` 構造体を追加し、`Config { Port, Database, OIDC }` 構成にする
3. `lint` & `test` パス (`make -C core lint test`)
4. **Codex レビュー実行** (コマンドは末尾に記載、対象 = 本ステップの diff)
5. 指摘対応 → 次のステップ

### ステップ 2: keystore パッケージ (KeySet I/F + staticKeySet + 起動時生成 + kid + 異常系)

1. テスト先 (`core/internal/keystore/keystore_test.go`):
   - **正常系**: PKCS#8 PEM 読み込み (RSA 2048 bit 動的生成 → PEM 化 → temp file → ロード) で kid 計算が決定論
   - **kid アルゴリズム**: 公開鍵 DER (`x509.MarshalPKIXPublicKey` 出力) の SHA-256 → 先頭 24 hex (F-11、RFC 7638 thumbprint **非準拠**)
   - **kid override**: `CORE_OIDC_KEY_ID` env 設定があれば自動算出より優先
   - **鍵長透過テスト**: RSA 1024 / 2048 / 3072 / 4096 bit を動的生成 → ロード → `Active` / `Verifying` がエラーなく機能
   - **異常系**:
     - PKCS#1 PEM (`-----BEGIN RSA PRIVATE KEY-----`) を渡したら `KEY_FORMAT_NOT_PKCS8` 内部 code 相当の明確エラー (本タスクは `apperror.CodedError` を使わずプレーン error で返す、論点 #9)
     - encrypted PEM (パスフレーズ付き) は非対応エラー、message に「復号済み PEM を K8s Secret に配置してください」を含める
     - 1024 bit 鍵は **拒否しない** が WARN ログ出力可能なフックを残す (P4 で main.go 側から呼び出される)
   - **起動時生成モード** (`CORE_OIDC_DEV_GENERATE_KEY=1`): `crypto/rsa.GenerateKey(rand.Reader, 2048)` でメモリ生成 → 同じ KeySet I/F でアクセス可能
2. `core/internal/keystore/keystore.go`:

   ```go
   type KeyPair struct {
       Kid        string
       PublicKey  *rsa.PublicKey
       PrivateKey *rsa.PrivateKey
       Alg        string // "RS256"
   }

   type KeySet interface {
       Active(ctx context.Context) (*KeyPair, error)
       Verifying(ctx context.Context) ([]*KeyPair, error)
   }

   type Source int
   const (
       SourceFile Source = iota
       SourceGenerated
   )

   func Init(ctx context.Context, cfg OIDCKeyConfig, l *logger.Logger) (KeySet, Source, error) { ... }
   ```

3. `staticKeySet` を内部実装 (1 鍵保持、`Active` と `Verifying` の両方が同じ鍵を返す)
4. `lint` & `test` パス
5. **Codex レビュー実行**
6. 指摘対応 → 次のステップ

### ステップ 3: devkeygen CLI (`core/cmd/devkeygen/main.go`)

1. テスト先 (`core/cmd/devkeygen/main_test.go`):
   - `-out <dir>` フラグで出力先指定、未指定は `./dev-keys/`
   - 既存ファイルがあれば `-force` フラグなしの場合エラー (上書き防止)
   - 出力ファイル: `signing.pem` (秘密鍵 PKCS#8 PEM) + `signing.pub.pem` (公開鍵 PKIX PEM)
   - 出力 mode: 秘密鍵 = `0600`、公開鍵 = `0644` (論点 #13、devkeygen 側で 0600 強制)
   - 鍵長: 2048 bit 固定 (`-bits` フラグなし、論点 #5)
2. 実装:

   ```go
   // main 内
   key, err := rsa.GenerateKey(rand.Reader, 2048)
   pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
   pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
   pem.Encode(privFile, &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})
   pem.Encode(pubFile, &pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
   os.WriteFile(privPath, ..., 0600)
   ```

3. `lint` & `test` パス
4. **Codex レビュー実行**
5. 指摘対応 → 次のステップ

### ステップ 4: Makefile + .gitignore

1. `core/Makefile` に `dev-keygen` ターゲット追加:
   ```makefile
   dev-keygen: ## dev 用 RSA 鍵を core/dev-keys/ に生成 (秘密鍵はコミット禁止)
       go run ./cmd/devkeygen -out ./dev-keys/
   ```
2. `core/Makefile` の `run` ターゲットで `CORE_ENV ?= dev` をデフォルトに設定 (Q7 確定)
3. `.gitignore` に `core/dev-keys/` を追加 (リポジトリルートの `.gitignore`)
4. `make -C core dev-keygen` で `core/dev-keys/signing.pem` + `signing.pub.pem` が生成され、permission が `0600`/`0644` であることを手動確認
5. `git status` で `core/dev-keys/` が untracked にも出ないことを確認 (`.gitignore` 反映確認)
6. **Codex レビュー実行** (Makefile + .gitignore 含む全 P1 範囲の最終 diff)
7. 指摘対応 → 次のステップ

### ステップ最終: 全体テスト + カバレッジ確認

1. `make -C core lint test` 緑 (config / keystore / devkeygen 単体テスト全件パス)
2. `make -C core test-cover` でカバレッジ確認:
   - keystore: 90% 以上 (異常系含む)
   - config: 90% 以上 (env 検証パス全件)
   - devkeygen: 80% 以上 (ファイル出力テスト含む)
3. 完了報告 (`docs/context/` 更新は P4 でまとめて行うため本タスクではスキップ)

## 実装コンテキスト

以下のファイルを読み取ってから実装を開始すること:

```
CONTEXT_DIR="docs/context"
```

- `${CONTEXT_DIR}/app/architecture.md` (全体構成)
- `${CONTEXT_DIR}/backend/conventions.md` (M0.3 までの規約: log.Fatal\* 不使用 / redact / 環境変数命名 / DB / マイグレーション)
- `${CONTEXT_DIR}/backend/patterns.md` (DB 接続 / マイグレーション運用 / 統合テスト / context ID 伝播)
- `${CONTEXT_DIR}/backend/registry.md` (パッケージ・環境変数一覧、本タスクで `CORE_OIDC_*` を追加するが registry の更新は P4 で行う)
- `${CONTEXT_DIR}/testing/backend.md` (テスト規約)

設計書: `docs/specs/32/index.md` (特に「主要な決定事項」「設計時の論点 #1, #5, #9, #10, #13, #14, #16」「環境変数 (新規追加)」「テスト観点」)

要求文書: `docs/requirements/32/index.md` (F-7, F-8, F-9, F-11, F-13, F-14, F-15, F-18)

適用範囲: `core/` のみ。`examples/...` には触らない

## 前提条件

- M0.3 完了 (DB 接続 + マイグレーション基盤、`testutil/dbtest`)
- 本タスクは P2 / P3 / P4 の前提 (鍵管理基盤がないと handler が動かない)
- 後続: P2 (Discovery) と P3 (JWKS + notimpl) は本タスク完了後に並列着手可能

## 不明点ハンドリング

- 矛盾・欠落・未定義がある場合は作業を止める
- 勝手な推測で実装を進めない
- 質問時は: 止まっている作業単位 / 判断が必要な論点 / 選択肢を整理して提示
- 例: 「OIDCConfig 構造体のフィールド名」「kid 計算で公開鍵のどのバイト列に SHA-256 をかけるか (`MarshalPKIXPublicKey` の出力で良いかの確証)」等で迷ったらユーザーに確認

## タスク境界

### 実装する範囲

- `core/internal/config/` の OIDC 関連拡張 (`OIDCConfig` 構造体、env 検証、issuer 正規化、endpoint override、Cache-Control max-age 範囲検証)
- `core/internal/keystore/` 新規パッケージ (`KeySet` I/F + `staticKeySet` + `Init` 関数 + kid 算出 + 鍵長透過 + 異常系)
- `core/cmd/devkeygen/` 新規 CLI (`main.go` + `main_test.go`、Go 標準ライブラリのみ、PKCS#8 PEM 出力、`0600` permission)
- `core/Makefile` の `dev-keygen` ターゲット追加 + `run` の `CORE_ENV ?= dev` デフォルト
- リポジトリルートの `.gitignore` に `core/dev-keys/` 追加

### 実装しない範囲 (後続タスク)

- Discovery handler 実装 (P2)
- JWKS handler 実装 / notimpl 503 stub (P3)
- main.go の起動シーケンス統合 / server.go への route 登録 / 起動 INFO/WARN ログ (P4)
- `docs/context/` の各ファイル更新 (conventions.md OIDC OP 規約節 / registry.md / patterns.md / testing/backend.md)、README 加筆 (P4)
- M2.x の鍵 rotation API、複数鍵保持の `multiKeySet` (本マイルストーン非実装)

## 設計仕様 (設計書から本タスク該当箇所を抜粋)

### 環境変数 (新規追加、本タスクで実装)

| 環境変数                           | 必須 | 既定値                | 用途                                                                                                     |
| ---------------------------------- | :--: | --------------------- | -------------------------------------------------------------------------------------------------------- |
| `CORE_ENV`                         |  ◯   | (なし)                | 環境識別子。`prod`/`staging`/`dev` のみ許容、それ以外は起動失敗                                          |
| `CORE_OIDC_ISSUER`                 |  ◯   | (なし)                | OP の論理識別子 URL。`prod`/`staging` で `https://` 必須、`dev` は `http://` 許可、末尾 `/` strip        |
| `CORE_OIDC_KEY_FILE`               |  △   | (なし)                | PEM PKCS#8 秘密鍵ファイルパス。`prod` で必須、`staging`/`dev` は `CORE_OIDC_DEV_GENERATE_KEY=1` で代替可 |
| `CORE_OIDC_DEV_GENERATE_KEY`       |  ×   | `0`                   | `1` で起動時 RSA 2048 bit 鍵生成 (メモリ保持)。`prod` では強制無効 (起動失敗)                            |
| `CORE_OIDC_KEY_ID`                 |  ×   | (自動算出)            | kid 固定値。未設定時は公開鍵 DER SHA-256 先頭 24 hex                                                     |
| `CORE_OIDC_JWKS_MAX_AGE`           |  ×   | `300`                 | JWKS Cache-Control max-age 秒 (0〜86400)                                                                 |
| `CORE_OIDC_DISCOVERY_MAX_AGE`      |  ×   | `0`                   | Discovery Cache-Control max-age 秒 (0 → no-cache、>0 → public, max-age)                                  |
| `CORE_OIDC_AUTHORIZATION_ENDPOINT` |  ×   | issuer + `/authorize` | endpoint 個別 override (設定があれば優先)                                                                |
| `CORE_OIDC_TOKEN_ENDPOINT`         |  ×   | issuer + `/token`     | 同上                                                                                                     |
| `CORE_OIDC_USERINFO_ENDPOINT`      |  ×   | issuer + `/userinfo`  | 同上                                                                                                     |
| `CORE_OIDC_JWKS_URI`               |  ×   | issuer + `/jwks`      | jwks_uri 個別 override                                                                                   |

### keystore I/F (確定済、論点 #10 反映)

```go
package keystore

type KeyPair struct {
    Kid        string             // F-11: 公開鍵 DER SHA-256 先頭 24 hex (RFC 7638 thumbprint 非準拠)
    PublicKey  *rsa.PublicKey
    PrivateKey *rsa.PrivateKey    // 起動時生成モードでは GenerateKey 結果、ファイルモードでは ParsePKCS8PrivateKey 結果
    Alg        string             // "RS256" 固定 (M1.1 範囲)
}

type KeySet interface {
    Active(ctx context.Context) (*KeyPair, error)         // 署名に使う「現在鍵」(M1.1 では 1 鍵のみ)
    Verifying(ctx context.Context) ([]*KeyPair, error)    // JWKS で広告する公開鍵 (M1.1 では 1 鍵のみ、M2.x で複数化)
}

type Source int
const (
    SourceFile Source = iota      // CORE_OIDC_KEY_FILE
    SourceGenerated               // CORE_OIDC_DEV_GENERATE_KEY=1
)

func Init(ctx context.Context, cfg OIDCKeyConfig, l *logger.Logger) (KeySet, Source, error)
```

### kid 算出 (F-11、論点 #10 確定)

- 計算式: `hex.EncodeToString(sha256.Sum256(x509.MarshalPKIXPublicKey(pubKey))[:12])` (12 バイト = 24 hex 文字)
- 注意: SHA-256 出力 32 バイトの先頭 12 バイト (= 24 hex 文字)、ETag (16 バイト = base64url) とはサイズが異なる
- `CORE_OIDC_KEY_ID` 設定時はその値を優先 (空文字でない場合のみ override)
- **RFC 7638 thumbprint 非準拠** (要求 F-11 確定済、変更しない)。ログ表記は「kid」または「fingerprint」とし「thumbprint」は使わない

### 異常系仕様 (論点 #10 Codex MEDIUM 2 反映)

| 入力                                                    | 挙動                                                                                                                                                      |
| ------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PKCS#1 PEM (`-----BEGIN RSA PRIVATE KEY-----`)          | エラー: 「鍵フォーマットが PKCS#8 ではありません。`openssl pkcs8 -topk8 -nocrypt -in <pkcs1> -out <pkcs8>` で変換してください」                           |
| encrypted PEM (`-----BEGIN ENCRYPTED PRIVATE KEY-----`) | エラー: 「encrypted PEM は非対応です。復号済み PEM を K8s Secret に配置してください」                                                                     |
| 1024 bit 以下の鍵                                       | 拒否しない、ロード成功 (本タスクではログを出さず、P4 で main.go 側から WARN ログ出力フックを呼ぶ。本タスクでは関数として `(KeyPair).BitLen() int` を提供) |
| 不正な PEM (Base64 デコード不能等)                      | エラー: 「PEM の解析に失敗しました: <原因>」                                                                                                              |
| RSA 以外の鍵 (EC / Ed25519 等)                          | エラー: 「RS256 では RSA 鍵が必要です。受け取った鍵タイプ: <type>」                                                                                       |

### Source ログ仕様 (P4 で main.go 側から呼ばれる、本タスクは I/F のみ提供)

`Init` の戻り値 `Source` (= `SourceFile` / `SourceGenerated`) を main.go の起動 INFO ログに渡す:

```
INFO 起動鍵情報 source=file kid=abcd1234... alg=RS256 env=dev
WARN dev 鍵生成モード: 単一 Pod 専用、複数 Pod 環境では CORE_OIDC_KEY_FILE で共有 Secret を使え (source=generated 時のみ)
```

## テスト観点 (本タスク該当のみ)

### config 単体

- `CORE_ENV` strict 検証: `prod` / `staging` / `dev` 受け入れ、`""` / `"production"` / `"PROD"` / `"local"` 等は起動失敗
- `CORE_OIDC_ISSUER` 正規化: `https://example.com/id-core/` → `https://example.com/id-core` (strip)、`http://localhost:8080` は `dev` のみ許可、`prod` で `http://` は失敗
- `CORE_OIDC_KEY_FILE` 必須性: `prod` + 未設定 → 失敗、`prod` + `CORE_OIDC_DEV_GENERATE_KEY=1` → 失敗、`dev` + 両方未設定 → 失敗
- `CORE_OIDC_JWKS_MAX_AGE` 範囲: `-1` / `86401` で失敗、`0` / `86400` で成功
- `CORE_OIDC_DISCOVERY_MAX_AGE` 範囲: 同上

### keystore 単体

- ファイルモード正常系: PKCS#8 PEM ロード → kid 計算 → `Active` / `Verifying` が同じ鍵を返す
- 起動時生成モード正常系: `CORE_OIDC_DEV_GENERATE_KEY=1` で 2048 bit RSA 生成 → kid 算出 → `Active` / `Verifying` が機能
- kid 決定論: 同じ公開鍵から常に同じ kid (100 回呼び出し全て一致)
- kid override: `CORE_OIDC_KEY_ID="custom-kid"` で `Active().Kid == "custom-kid"`
- 鍵長透過: RSA 1024 / 2048 / 3072 / 4096 bit を動的生成して全てロード成功 (4096 はテスト時間が長くなるので `-short` フラグでスキップ可能なテスト関数に分離)
- 異常系: PKCS#1 / encrypted PEM / EC 鍵 / 不正 PEM / 空ファイル / 存在しないファイルパス
- `BitLen()` メソッドが期待値を返す (1024 → 1024、2048 → 2048 等)

### devkeygen 単体

- 既定出力先 (`./dev-keys/`) に `signing.pem` + `signing.pub.pem` が生成
- `-out <custom_dir>` フラグで出力先変更
- 出力ファイルの permission: `signing.pem = 0600`、`signing.pub.pem = 0644`
- 既存ファイルがあれば `-force` なしでエラー、`-force` で上書き
- 出力 PEM が keystore でロード可能 (devkeygen → keystore.Init の round-trip テスト)
- 鍵長 = 2048 bit (出力 PEM をロードして `BitLen() == 2048` 確認)

## Codex レビューコマンド (各ステップで使用)

```bash
codex exec --full-auto -c model_reasoning_effort="medium" \
  "まず以下の context ファイルを読み取れ:
   - docs/context/app/architecture.md
   - docs/context/backend/conventions.md
   - docs/context/backend/patterns.md
   - docs/context/backend/registry.md
   - docs/context/testing/backend.md
   設計書: docs/specs/32/index.md (特に論点 #1, #5, #9, #10, #13, #14, #16 と環境変数表)
   要求文書: docs/requirements/32/index.md (F-7, F-8, F-9, F-11, F-13, F-14, F-18)

   その上で git diff をレビューせよ。

   Check (本タスクは config + keystore + devkeygen + Makefile + .gitignore):
   1) TDD compliance (テスト先行 / カバレッジ 90%+)
   2) CORE_ENV strict 検証の網羅 (prod/staging/dev 以外 / 空 / unset 全パターン)
   3) issuer 正規化の正しさ (https 必須 + dev http 許可 + 末尾 strip)
   4) keystore I/F の契約遵守 (KeySet { Active, Verifying } シグネチャ、staticKeySet が両方で同じ鍵を返す)
   5) kid 算出の決定論性 (公開鍵 DER SHA-256 先頭 24 hex、RFC 7638 thumbprint と混同していないか)
   6) 異常系網羅 (PKCS#1 / encrypted PEM / EC 鍵 / 不正 PEM、明確なエラー文言)
   7) 鍵長透過 (1024/2048/3072/4096 bit すべてロード成功)
   8) devkeygen の permission (秘密鍵 0600 強制、Go 標準 os.WriteFile mode)
   9) .gitignore 反映確認 (core/dev-keys/ が untracked にも出ない)
   10) log.Fatal* 不使用、UUID v7 規約、Co-Authored-By trailer 不使用
   11) 探索禁止違反がないか (Grep / Glob / Explore agent を使っていないか)

   Report issues as CRITICAL/HIGH/MEDIUM/LOW in Japanese. Last section must be ## Summary with counts and gate verdict (CRITICAL=0/HIGH=0/MEDIUM<3 → PASS)."
```

各ステップ完了時にこのコマンドを実行し、ゲート未達時は修正サイクル (修正 → push → 再レビュー) を回す。

## 完了条件

- [ ] ステップ 1: config パッケージ拡張 (`OIDCConfig` 追加 + 全 env 検証 + テスト) 完了 + Codex ゲート PASS
- [ ] ステップ 2: keystore パッケージ (KeySet I/F + staticKeySet + kid + 異常系 + 鍵長透過テスト) 完了 + Codex ゲート PASS
- [ ] ステップ 3: devkeygen CLI (`core/cmd/devkeygen/`) + permission 0600 + テスト round-trip 完了 + Codex ゲート PASS
- [ ] ステップ 4: Makefile (`dev-keygen` ターゲット + `CORE_ENV ?= dev`) + `.gitignore` (`core/dev-keys/`) 完了 + Codex ゲート PASS
- [ ] `make -C core lint test` 緑、カバレッジ keystore/config 90%+ / devkeygen 80%+
- [ ] `make -C core dev-keygen` で実際に PEM 生成 + permission 確認 + `git status` で untracked にも出ないこと確認
- [ ] PR 作成 (`/pr-codex-review {番号}` で最終 Codex レビュー、CRITICAL=0/HIGH=0/MEDIUM<3 で main にマージ)
- [ ] `docs/context/` 更新は本タスクではスキップ (P4 で集約)
- [ ] 未解決の仕様質問が残っていない
