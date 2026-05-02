# 要求 #32: OIDC Discovery + JWKS エンドポイントを core/ に導入

- 関連 Issue: [#32](https://github.com/mktkhr/id-core/issues/32)
- マイルストーン: [M1.1: OIDC Discovery + JWKS エンドポイント](https://github.com/mktkhr/id-core/milestone/4)
- 状態: 設計着手OK
- 起票日: 2026-05-02
- 最終更新: 2026-05-02

## 背景・目的

id-core を OIDC OP として外部の RP (Relying Party) が認識・接続できる最小骨格を整える。Phase 1 (M1.x: OIDC OP 最小コア) の入り口として、後続の認可 (M1.2) / トークン (M1.3) / UserInfo (M1.4) / RP 接続 E2E (M1.5) を実装する前提となる「**メタデータ公開**」と「**公開鍵公開**」を確立する。

このマイルストーンが完了しない限り、以下が実現できない:

- M1.2: RP がメタデータから `authorization_endpoint` を発見できない
- M1.3: RP がメタデータから `token_endpoint` を発見できない、ID Token 検証用の公開鍵を取得できない
- M1.4: RP がメタデータから `userinfo_endpoint` を発見できない
- M1.5: RP の OIDC ライブラリ (例: go-oidc) が Discovery 経路で id-core を初期化できない

スコープは**メタデータ公開 + 公開鍵公開 + 鍵管理基盤**であり、認可コード発行・トークン発行・ID Token 署名処理は対象外 (M1.2 以降で実装)。

起動前提は **K8s** (HPA で複数 Pod 想定)。Pod 再起動・スケールイン/アウトでも JWKS が不整合にならないこと、reverse proxy / Ingress 経由の TLS 終端を前提にすることが必須。

## ユーザーシナリオ

### 開発者 (RP 側、id-core を OP として接続したい)

1. RP の OIDC クライアントライブラリに **`issuer = https://id.example.com`** を渡す
2. クライアントが `GET https://id.example.com/.well-known/openid-configuration` を叩いてメタデータを取得
3. メタデータから `jwks_uri` を取得し、`GET <jwks_uri>` で JWKS を取得
4. 取得した JWKS の公開鍵を ID Token 検証時に利用 (M1.3 以降)
5. M1.1 範囲では実エンドポイント (`/authorize`, `/token`, `/userinfo`) は**未実装** (Discovery で広告だけはする予告含む)

### 運用者 (id-core を K8s 上で運用)

1. K8s Secret に PEM 形式の RSA 秘密鍵を保存し、Pod の `/etc/id-core/keys/signing.pem` にマウント
2. Pod 環境変数 `CORE_OIDC_KEY_FILE=/etc/id-core/keys/signing.pem` を設定
3. Pod 環境変数 `CORE_OIDC_ISSUER=https://id.example.com` を設定
4. id-core 起動時にログで「鍵 fingerprint = `xxxxxxxx...` を読み込みました」を確認
5. ローリングアップデート時、Secret 不変なら全 Pod が同一鍵で署名 → JWKS 不整合なし
6. Secret を更新する場合は K8s 標準の rolling restart で順次反映される (M2.x で多鍵 rotation を本格化、M1.1 では単一鍵運用前提)

### 開発者 (id-core 自身を開発)

1. `make dev-keygen` でローカルに dev 用 RSA 鍵ペアを `core/dev-keys/` に生成 (`.gitignore` で除外、コミットされない)
2. `docker compose up -d postgres` で DB 起動 (M0.3 同様)
3. `CORE_ENV=dev CORE_OIDC_ISSUER=http://localhost:8080 CORE_OIDC_KEY_FILE=./dev-keys/signing.pem make -C core run` (cwd=`core/` 前提)
4. `curl http://localhost:8080/.well-known/openid-configuration` でメタデータを確認
5. `curl http://localhost:8080/jwks` で JWKS を確認

## 用語

- **要求文書**: 本ファイル (`docs/requirements/32/index.md`)
- **OP (OpenID Provider)**: OIDC で ID Token を発行する側 (id-core はこちら)
- **RP (Relying Party)**: OP に認証を委譲する側 (M1.5 で go-react サンプルが該当)
- **Discovery**: OP のメタデータ (endpoint URL / 対応 grant / 対応 algorithm 等) を `GET /.well-known/openid-configuration` で公開する仕組み (OpenID Connect Discovery 1.0 / RFC 8414)
- **JWKS (JSON Web Key Set)**: 公開鍵を JSON 形式で公開する仕組み (RFC 7517)
- **kid (Key ID)**: JWKS の各鍵に付与する識別子。署名 JWT の `kid` ヘッダで検証側が鍵を選択する
- **issuer**: OP の論理識別子 (URL 形式)。RP の OIDC ライブラリの初期化に使用
- **規約書**: `docs/context/backend/conventions.md` の OIDC OP 規約節 (本要求で新設)

## 編集ルール (本ファイル限定)

- 本ファイル内では `@` 記号を裸で書かない (`@v1` / `@master` / `@user` 等)。GitHub の自動メンション混入を防ぐため、必ずバッククォートで囲むか別表現に言い換える
- 既存の規約は [.rulesync/rules/pr-review-policy.md](../../../.rulesync/rules/pr-review-policy.md) §5 を参照

## 機能要件

| ID       | 内容                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- |
| F-1      | `GET /.well-known/openid-configuration` が **OpenID Connect Discovery 1.0 / RFC 8414 準拠**の JSON metadata を返す。HTTP 200 + `Content-Type: application/json`。`issuer` フィールドが `CORE_OIDC_ISSUER` env と完全一致する。レスポンス Cache-Control は `CORE_OIDC_DISCOVERY_MAX_AGE` env で制御 (Q15 確定): 既定 `0` のとき `no-cache, must-revalidate`、`> 0` のとき `public, max-age=<N>, must-revalidate` (どちらか一方の指定で `no-cache` と `max-age` が併記される矛盾を避ける)。ETag は常時付与                                                    |
| F-2      | Discovery メタデータに以下のフィールドが含まれる: `issuer` / `authorization_endpoint` / `token_endpoint` / `userinfo_endpoint` / `jwks_uri` / `response_types_supported` / `grant_types_supported` / `subject_types_supported` / `id_token_signing_alg_values_supported` / `scopes_supported` / `token_endpoint_auth_methods_supported`                                                                                                                                                                                                                     |
| F-3      | 各エンドポイント URL (`authorization_endpoint` 等) は `${ISSUER}` をベースに `url.URL.JoinPath` 相当で組み立て、**issuer に path を含むケース** (例: `https://example.com/id-core`) でも正しく動作する。各エンドポイントは個別 env (`CORE_OIDC_AUTHORIZATION_ENDPOINT` 等) で override 可能                                                                                                                                                                                                                                                                 |
| F-4      | M1.1 時点の Discovery 広告内容: `scopes_supported` = `["openid"]`、`response_types_supported` = `["code"]`、`grant_types_supported` = `["authorization_code"]`、`id_token_signing_alg_values_supported` = `["RS256"]`、`subject_types_supported` = `["public"]`、`token_endpoint_auth_methods_supported` = `["client_secret_basic"]`。後続マイルストーンで実装が進む際に追加する                                                                                                                                                                            |
| F-5      | `GET /jwks` (Q9 確定) が **JWKS (RFC 7517) 形式**の JSON を返す。`{"keys": [...]}` 配列形式で、M1.1 時点では 1 鍵のみ含む。各鍵には `kty` / `use` / `alg` / `kid` / `n` / `e` (RSA の場合) を含む。Discovery の `jwks_uri` でこの URL を広告                                                                                                                                                                                                                                                                                                                |
| F-6      | JWKS レスポンスに `Cache-Control: public, max-age=<X>, must-revalidate` および `ETag` ヘッダを付与する (Q4 確定)。max-age の既定値は **300 秒 (5 分)**、`CORE_OIDC_JWKS_MAX_AGE` env で 0〜86400 の範囲で override 可能。`immutable` は使わない (M2.x で鍵 rotation 予定のため)。RP ライブラリの多くは独自再取得ロジックを持つため Cache-Control は補助、本体は M2.x の overlap window 設計                                                                                                                                                                 |
| F-7      | 鍵保管方式: **本番**は `CORE_OIDC_KEY_FILE` env で K8s Secret マウントパスを指定 (PEM 形式 = **PKCS#8**、Q1 確定)。**開発・テスト**は `CORE_OIDC_DEV_GENERATE_KEY=1` で起動時生成 (メモリ保持、`CORE_ENV != prod` のときのみ許可)                                                                                                                                                                                                                                                                                                                           |
| F-8      | 起動時生成モード (`CORE_OIDC_DEV_GENERATE_KEY=1`) で起動した時、構造化 **WARN ログ**で「dev 鍵生成モード = 単一 Pod 専用、複数 Pod 環境では `CORE_OIDC_KEY_FILE` で共有 Secret を使え」と明示する。**複数 Pod ガード自体は Helm chart / K8s manifest の運用側責任** (アプリ側は env でレプリカ数を検証しない、責務分界の判断、Q5 確定)。Codex CRITICAL 指摘 (複数 Pod 別鍵生成) は運用側 template で `replicas: 1` を強制することで担保                                                                                                                     |     |
| F-9      | `CORE_ENV` は **strict 3 値** (`prod` / `staging` / `dev`)、それ以外 (空文字 / unset 含む) は起動失敗 (Q7 確定)。`CORE_ENV=prod` のとき、`CORE_OIDC_KEY_FILE` 未設定で起動を許容しない (fail-fast)。`CORE_OIDC_DEV_GENERATE_KEY=1` も `CORE_ENV=prod` では無効 (起動失敗)。`Makefile` の `run` ターゲットで `CORE_ENV ?= dev` をデフォルトに、K8s manifest は明示設定必須                                                                                                                                                                                   |
| ~~F-10~~ | **削除** (Q6 / Q11 と連動): 当初 Codex 指摘で「既知 dev 鍵 fingerprint を prod 検知して fail-fast」を追加したが、id-core ではリポジトリに固定 dev 鍵をコミットしない方針 (Q2 / `.gitignore`) のため検知すべき固定 fingerprint が存在しない。F-9 (prod で `CORE_OIDC_KEY_FILE` 必須化 + 起動時生成モード無効化) で実害防止できるため二重ガードは不要と判断                                                                                                                                                                                                   |
| F-11     | 鍵の `kid` (Key ID) は `CORE_OIDC_KEY_ID` env で固定可能。未設定時は公開鍵 SHA-256 hash の先頭 24 hex で自動生成。生成ロジックは決定論的 (同じ公開鍵から常に同じ kid)。**運用ログ上の鍵 fingerprint は kid と同値**として扱う (運用者は `kid` 値で起動鍵を一意識別できる、F-18 と整合)                                                                                                                                                                                                                                                                      |
| F-12     | 署名アルゴリズムは **RS256 のみ**で開始。設定構造は配列値で保持し、M2.x 以降で ES256 / EdDSA を追加可能 (M1.1 では実装しないが I/F 予告)。keystore 層は `KeySet` interface で `Active(ctx)` / `Verifying(ctx)` を提供 (Q10 確定、M1.1 では内部 1 鍵保持の `staticKeySet` 実装)。M2.x の rotation 実装は `Verifying()` を「現在 + 過去鍵」を返すよう変更するだけで完了                                                                                                                                                                                       |     |
| F-13     | dev key 生成は `make dev-keygen` ターゲットで実行する。実装は **Go 標準 `crypto/rsa` + `crypto/x509`** (Q2 確定) で `core/cmd/devkeygen/main.go` に置き、`go run ./cmd/devkeygen -out ./dev-keys/` 形式で呼ぶ。openssl 依存なし。出力先は `core/dev-keys/` (`.gitignore` で除外、リポジトリにコミットしない)                                                                                                                                                                                                                                                |
| F-14     | リポジトリには **dev 鍵 (秘密鍵 / 公開鍵とも) を一切コミットしない**。dev 鍵は各開発者が `make dev-keygen` で生成 (Q2 確定)。test 用の鍵はテスト関数内で動的生成 (`crypto/rsa.GenerateKey`) してメモリ保持で完結する。リポジトリ同梱物としての固定 dev fingerprint は持たない                                                                                                                                                                                                                                                                               |
| F-15     | `core/cmd/core/main.go` 起動シーケンスに鍵読み込み + 環境ガードを統合する。M0.3 で確立したシーケンス (logger → config → ctx → db.Open → AssertClean → server) の「config」と「server」の間に keystore 初期化を挿入。失敗時は `logger.Error` + `os.Exit(1)` (`log.Fatal*` 不使用ポリシーは踏襲)                                                                                                                                                                                                                                                              |
| F-16     | `/.well-known/openid-configuration` および `/jwks` は **認証不要** (公開エンドポイント)。`/health/live` / `/health/ready` (M0.3) と並ぶ 0 認証層に位置付ける                                                                                                                                                                                                                                                                                                                                                                                                |
| F-17     | エンドポイント URL の **path rewrite / subpath 配置 (Ingress 経由)** に対応する。`${ISSUER}=https://example.com/id-core` のケースでも各エンドポイント (`https://example.com/id-core/authorize` 等) が正しく組み立つ。Q8 確定の **テーブル駆動 5 ケース** (標準 / subpath / 末尾スラッシュ / dev / 非標準ポート) で ContractTest を持つ                                                                                                                                                                                                                      |
| F-18     | Discovery / JWKS の構造化ログには `request_id` (HTTP 経路は M0.2 middleware で自動付与) を含める。**鍵読み込みログ**には `kid` 値を fingerprint として含む (F-11 と同値: 公開鍵 SHA-256 先頭 24 hex)。**秘密鍵そのもの・PEM フルダンプ・公開鍵 modulus / exponent そのものは出力禁止** (M0.3 の F-10 redact 方針を踏襲)                                                                                                                                                                                                                                     |
| F-19     | M1.1 の規約書 (`docs/context/backend/conventions.md` の OIDC OP 規約節新設) に最低必須項目を含む: (1) issuer URL の規約、(2) 環境変数一覧 (`CORE_OIDC_*` / `CORE_ENV`)、(3) 鍵保管方式 (本番 / dev の分岐) + **kid と fingerprint の関係 (F-11 / F-18 同値)**、(4) **dev 鍵生成モードの単一 Pod 制約 (Helm/manifest 側で `replicas: 1` を強制する旨を明記、実 sample は M1.5 で整備)**、(5) 開発者向け運用手順 (make dev-keygen / docker compose / make run)、(6) **M1.1 の鍵更新運用制約 (F-24: Pod 全停止 → 起動) と M2.x の overlap window 予告 (F-22)** |     |
| F-20     | テストカバレッジの最低ライン: (a) Discovery / JWKS の HTTP レスポンス JSON 構造の単体テスト、(b) issuer に path を含むケースの URL 構築 ContractTest (F-17)、(c) `CORE_ENV=prod` + `CORE_OIDC_KEY_FILE` 未設定の起動失敗統合テスト (F-9)、(d) 起動時生成モードでの kid 自動生成テスト                                                                                                                                                                                                                                                                       |
| F-21     | JWKS の JSON シリアライズは **決定的** (deterministic) であり、ETag 値が安定する。鍵セットが同一なら全 Pod / 全リクエストで同一バイト列を返す (キー順序固定 / 空白ポリシー固定)。これにより K8s 複数 Pod 環境でも ETag が Pod 間で一貫し、RP 側の `If-None-Match` が機能する                                                                                                                                                                                                                                                                                |
| F-22     | M2.x の鍵 rotation 時の **overlap window** (新旧鍵併存期間) を `>= max-age + RP 再取得遅延 + 時計ずれバッファ` で設計する旨を、規約書に**予告として明記**する。M1.1 では実装しないが、Cache-Control max-age = 5 分の場合の overlap window 実務目安は 15〜30 分以上 (Codex セカンドオピニオンでの指針)                                                                                                                                                                                                                                                       |     |
| F-23     | M1.1 で広告するが M1.2-1.4 で実装される endpoint (`authorization_endpoint` / `token_endpoint` / `userinfo_endpoint`) は **OIDC Discovery 1.0 で REQUIRED のため Discovery メタデータには必須記載**。実 endpoint は M1.x の各マイルストーン到達まで HTTP **`503 Service Unavailable`** を返し、レスポンス body は機械可読 JSON (`{"error":"endpoint_not_implemented","available_at":"M1.2"}` 形式) とする。これにより RP は能力宣言と現状の不一致を確実に判別できる (Codex HIGH 反映)                                                                        |     |
| F-24     | M1.1 範囲では **鍵更新を非サポート**。Secret 内の鍵を変更したい場合は **Pod 全停止 → 新鍵で再起動** の運用フロー必須 (zero-downtime rotation は M2.x で実装)。M1.1 で rolling update 中に Secret を変更すると、Pod ごとに旧鍵/新鍵が混在し署名検証不整合を起こすリスクが顕在化するため、規約書に運用制約として明記する (Codex HIGH 反映)                                                                                                                                                                                                                    |     |

## 非機能要件

| 区分           | 要件                                                                                                                                                                                                                                                                                            |
| -------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| パフォーマンス | M1.1 では計測対象外。JWKS の `Cache-Control` max-age (Q4) で RP 側のキャッシュ期間を確保し、Discovery / JWKS リクエストの頻度を抑制する設計のみ要求                                                                                                                                             |
| セキュリティ   | 秘密鍵をログ・stderr に出さない (F-18 redact)。`CORE_ENV=prod` での `CORE_OIDC_KEY_FILE` 必須化 + 起動時生成モード無効化 (F-9) で誤運用防止。複数 Pod での生成モード暴発は Helm/manifest 側で `replicas: 1` 強制 (F-8 + Q5 確定)。host header / `X-Forwarded-Proto` を信頼しない (Ingress 前提) |
| 可用性         | Discovery / JWKS は静的レスポンス (鍵が変わらない限り同じ JSON) のため、応答時間は無視できる。鍵読み込み失敗で起動拒否 → K8s が再起動するまで Pod は Ready にならない (M0.3 の F-13 start gate と整合)                                                                                          |
| 保守性         | keystore 層と Discovery / JWKS 層を分離する (`core/internal/keystore/` と `core/internal/oidc/` または `core/internal/discovery/`)。M2.x の鍵 rotation 導入時に keystore 層のみを拡張すればよい構造                                                                                             |
| ポータビリティ | OS / アーキテクチャ依存コードを書かない。鍵生成は Go 標準ライブラリのみ (F-13)。openssl / 外部 CLI ツールに依存しない                                                                                                                                                                           |
| 再現性         | dev 起動時生成モードで生成された鍵は **メモリ保持のみ**で永続化されない (Pod 再起動で別鍵)。これは複数 Pod 環境で許容されないため、F-8 でガード。本番運用では K8s Secret 経由で鍵が静的に固定される                                                                                             |
| 検証性         | F-1〜F-20 を Go 単体テストおよび統合テスト (実機 K8s 不要、`CORE_ENV` env 切替 + replica count 検証で代替) で検証可能。CI 上で同等のテストが green になる                                                                                                                                       |
| 観測性         | 起動時に「鍵 fingerprint / kid / アルゴリズム / 取得経路 (file / 起動時生成) / `CORE_ENV` 値」を構造化 INFO ログで出力する。運用者がどの鍵で起動しているかを確認できる                                                                                                                          |

## 認可要件

本マイルストーンは認可制御を扱わない (Discovery / JWKS は **公開エンドポイント、認証不要**、F-16)。

`docs/context/authorization/matrix.md` には Discovery / JWKS の認可セルが存在しない (公開エンドポイントは認可マトリクス対象外という暗黙のポリシー) 想定。マスター更新は不要。

| 機能                                    | 全ロール (未認証含む) |
| --------------------------------------- | --------------------- |
| `GET /.well-known/openid-configuration` | o (公開、認証不要)    |
| `GET /jwks`                             | o (公開、認証不要)    |

## スコープ外 (含まないこと)

- 認可エンドポイント `/authorize` の実装 → M1.2
- トークンエンドポイント `/token` の実装 → M1.3
- UserInfo エンドポイント `/userinfo` の実装 → M1.4
- ID Token 署名処理本体 → M1.3
- 認可コード発行処理 → M1.2
- クライアント登録 → M2.1
- 鍵 rotation API (多鍵切替) → M2.x 以降 (本マイルストーンでは I/F のみ予告)
- 上流 IdP との連携 (id-core が RP) → M4.x
- 監査ログ (鍵読み込み・JWKS 配信の追跡) → M7.x
- **K8s manifest sample / Helm chart sample の整備** → M1.5 (E2E with go-react RP)。M1.1 の規約書には dev 鍵生成モードの運用制約 (Helm 側で `replicas: 1` を強制) のみ明記する

## 検討事項 (設計フェーズで論点化)

| #       | 論点                                                                | 第一候補                                                                                                                                                                                                                                                                 |
| ------- | ------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Q1      | 鍵フォーマット (PEM PKCS#1 / PKCS#8)                                | ✅ **確定: PKCS#8** — アルゴリズム非依存、M2.x の ES256/EdDSA 拡張時に keystore 層を変更せず済む                                                                                                                                                                         |
| Q2      | dev key 生成方法 (`make dev-keygen` の実装、openssl / Go 標準)      | ✅ **確定: Go 標準 `crypto/rsa` + `crypto/x509`** — `core/cmd/devkeygen/` 新設、openssl 不要                                                                                                                                                                             |
| Q3      | 鍵読み込みライブラリ (lestrrat-go/jwx / go-jose / 標準のみ)         | ✅ **確定: `github.com/lestrrat-go/jwx/v3`** — JWKS / JWT / JWS / JWE 網羅、M1.3 (ID Token 署名) / M4.x (上流 RP 連携) でも使い回し                                                                                                                                      |
| Q4      | JWKS の Cache-Control max-age                                       | ✅ **確定: 既定 5 分 (300s) + must-revalidate**、`CORE_OIDC_JWKS_MAX_AGE` env で 0〜86400 の範囲で override 可能、`CORE_ENV=dev` 時はキャッシュ抑制推奨                                                                                                                  |
| Q5      | `CORE_REPLICA_COUNT` の取得方法 (env 受け / Downward API / k8s API) | ✅ **確定: アプリ側は WARN ログのみ、複数 Pod ガードは Helm/manifest 運用側責任**。M1.1 では規約書に運用制約を明記、実 manifest / Helm chart sample の整備は M1.5 (E2E with RP) に送る                                                                                   |
| ~~Q6~~  | ~~既知 dev 鍵 fingerprint の保管場所~~                              | ❌ **削除** (F-10 廃止に連動): 検知すべき固定 fingerprint が不要のため論点自体が消滅                                                                                                                                                                                     |
| Q7      | `CORE_ENV` 値の正規化 (`prod` / `staging` / `dev` の 3 値 strict)   | ✅ **確定: 完全 strict** — `prod` / `staging` / `dev` 以外 (空文字 / unset 含む) は起動失敗。`Makefile` の `run` ターゲットで `CORE_ENV ?= dev` をデフォルトに、K8s manifest は明示設定必須                                                                              |
| Q8      | endpoint URL の path rewrite ContractTest 方針                      | ✅ **確定: テーブル駆動テスト 5 ケース最低** — (1) 標準 `https://id.example.com`、(2) subpath `https://example.com/id-core`、(3) 末尾スラッシュ `https://example.com/id-core/`、(4) dev `http://localhost:8080`、(5) 非標準ポート `https://id.example.com:9443`          |
| Q9      | `/jwks.json` か `/jwks` か (path 命名)                              | ✅ **確定: `/jwks`** (Codex セカンドオピニオン反映) — 大手 OP (Google / Microsoft / Okta / Keycloak) は拡張子なし優勢、REST 原則と整合、Content-Type 将来拡張に中立。Discovery の `jwks_uri` で動的広告するため RP 互換性は影響なし                                      |
| Q10     | 鍵 rotation API (M1.1 では実装しないが I/F 予告)                    | ✅ **確定: M1.1 から KeySet I/F で設計、内部 1 鍵保持** — `Active(ctx) (*KeyPair, error)` / `Verifying(ctx) ([]*KeyPair, error)` を提供、M2.x で rotation 実装時に I/F 変更不要                                                                                          |
| ~~Q11~~ | ~~JWKS への dev 鍵 fingerprint 検証テスト~~                         | ❌ **削除** (F-10 廃止に連動): F-9 (prod での `CORE_OIDC_KEY_FILE` 必須化) の起動失敗統合テストに置換                                                                                                                                                                    |
| Q12     | `client_secret_basic` 以外の認証方式の広告タイミング                | ✅ **確定: M1.1 は `client_secret_basic` のみ広告**。`client_secret_post` は M1.3 (token endpoint 実装) で post も RFC 6749 の SHOULD なので追加検討、`private_key_jwt` / `none` は M2.1 以降のクライアント登録 DB 拡張時に追加                                          |
| Q13     | issuer の正規化 (末尾スラッシュ有無、scheme 強制 https)             | ✅ **確定: https 必須 (`CORE_ENV=dev` のときのみ http 許可)、末尾スラッシュは strip して保持** — `CORE_ENV=prod`/`staging` では `https://` 始まり必須、`dev` では `http://` も許可。URL parse 不能なら起動失敗。ID Token `iss` claim との完全一致を担保                  |
| Q14     | OIDC エンドポイントへの middleware チェーン適用方針                 | ✅ **確定: M0.2 D1 順序 (`request_id → access_log → recover → handler`) を踏襲** — Discovery / JWKS も既存 `/health` 系と同じ middleware チェーンに乗せる。M1.2 (/authorize) / M1.3 (/token) 特有の middleware (rate limiting / CSRF 等) は D1 順序の内側に M1.2+ で追加 |
| Q15     | `discovery_endpoint` の Cache-Control 方針 (JWKS とは別判断)        | ✅ **確定: `Cache-Control: no-cache, must-revalidate` + ETag**。`CORE_OIDC_DISCOVERY_MAX_AGE` env で 0〜86400 の範囲で override 可能 (既定 `0` = no-cache 相当)。Discovery 変更 (新エンドポイント追加等) の即時反映を重視、ETag で 304 軽量化                            |

## 要求フェーズ状況

| フェーズ              | 状態   | 備考                                                                                                                             |
| --------------------- | ------ | -------------------------------------------------------------------------------------------------------------------------------- |
| 1. ドラフト           | 完了   | 2026-05-02、Issue #32 起点 + Codex セカンドオピニオン反映                                                                        |
| 2. 認可マトリクス突合 | 完了   | 2026-05-02、Discovery / JWKS は公開エンドポイントで認可マトリクス対象外と確認、マスター更新不要                                  |
| 3. 未決事項解決       | 完了   | 2026-05-02 完了、Q1〜Q5 + Q7〜Q10 + Q12〜Q15 確定 (13 件) + Q6/Q11 削除 (2 件) = 15/15 解決                                      |
| 4. レビュー           | 完了   | 2026-05-02 完了、Codex セカンドオピニオン (HIGH=2 / MEDIUM=4 / LOW=1) 全件反映、F-23 / F-24 追加 + F-1 / F-11 / F-18 / F-19 修正 |
| 5. 公開ゲート         | 公開済 | 2026-05-02、validate 合格、CRITICAL=0 / HIGH=0 / MEDIUM=0 でゲート通過、Issue #32 同期 + `状態:設計着手OK` ラベル付与            |

## 公開記録

- 公開日時: 2026-05-02
- Issue: https://github.com/mktkhr/id-core/issues/32
- ゲート結果:
  - validate: 合格
  - CRITICAL: 0 / HIGH: 0 / MEDIUM: 0
- 次工程: `/spec-pickup 32` で設計担当が引き取り → `/spec-create` または `/spec-full`

## 関連

- 親 Phase: Phase 1 (OIDC OP 最小コア)
- 後続要求: M1.2 (`/authorize`) → M1.3 (`/token`) → M1.4 (`/userinfo`) → M1.5 (E2E)
- 先行マイルストーン: M0.2 (ログ・エラー・middleware), M0.3 (DB 接続 + マイグレーション基盤)
- 関連スキル: `/backend-security` (鍵管理 / OIDC OP セキュリティ), `/backend-architecture`, `/backend-api-endpoint`
- 参照仕様:
  - OpenID Connect Discovery 1.0
  - RFC 8414 (OAuth 2.0 Authorization Server Metadata)
  - RFC 7517 (JSON Web Key)
  - RFC 7518 (JSON Web Algorithms)
  - OpenID Connect Core 1.0

## 変更履歴

| 日付       | 変更内容                                                                                                                                                                                                                                                                                                                                         |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --- |
| 2026-05-02 | 起票 (Issue #32 起点)。M0.3 完了確認後、M1.1 着手。Codex セカンドオピニオン (CRITICAL=1 / HIGH=3 / MEDIUM=3) 全件反映: dev 鍵運用制約 / endpoint URL JoinPath / 環境ガード / CORE_ENV 導入等                                                                                                                                                     |
| 2026-05-02 | Q1 確定: 鍵フォーマット = **PKCS#8** (アルゴリズム非依存、M2.x で ES256/EdDSA 追加時の互換性確保)                                                                                                                                                                                                                                                |
| 2026-05-02 | Q2 確定: dev key 生成 = **Go 標準 `crypto/rsa` + `crypto/x509`** (`core/cmd/devkeygen/`、openssl 依存なし、生成と読込を同じライブラリで担保)                                                                                                                                                                                                     |
| 2026-05-02 | Q3 確定: 鍵読み込み / JWKS シリアライザ = **`github.com/lestrrat-go/jwx/v3`** (M1.3 / M4.x で同ライブラリ流用、generics 対応で型安全)                                                                                                                                                                                                            |
| 2026-05-02 | Q4 確定 (Codex セカンドオピニオン反映): JWKS Cache-Control = **`public, max-age=300, must-revalidate`**、`CORE_OIDC_JWKS_MAX_AGE` env で override 可能、F-21 (決定的 JSON シリアライズ) / F-22 (M2.x overlap window 予告) 追加                                                                                                                   |
| 2026-05-02 | Q5 確定 (責務分界の見直し): **アプリ側は WARN ログのみ、複数 Pod ガードは Helm/manifest 運用側責任**。F-8 を改修 (env `CORE_REPLICA_COUNT` 廃止)、Helm/manifest sample 整備は M1.5 (E2E) に送る。Codex CRITICAL 指摘は運用側 template で担保                                                                                                     |
| 2026-05-02 | Q6 / Q11 削除 + F-10 削除 + F-14 簡素化: dev 鍵を一切リポジトリにコミットしない方針のため固定 fingerprint が存在しない。F-9 (prod で `CORE_OIDC_KEY_FILE` 必須化 + 起動時生成モード無効化) で実害防止できるため二重ガード (F-10) は overengineering と判断。test 鍵はテスト内で動的生成                                                          |
| 2026-05-02 | Q7 確定: `CORE_ENV` = **完全 strict 3 値** (`prod` / `staging` / `dev`)、空文字 / unset 含む不正値は起動失敗。`Makefile` の `run` ターゲットで `CORE_ENV ?= dev` デフォルト、K8s manifest は明示設定必須                                                                                                                                         |
| 2026-05-02 | Q8 確定: endpoint URL ContractTest = **テーブル駆動テスト 5 ケース最低** (標準 / subpath / 末尾スラッシュ / dev / 非標準ポート)                                                                                                                                                                                                                  |
| 2026-05-02 | Q9 確定 (Codex セカンドオピニオン反映): JWKS path = **`/jwks`** (拡張子なし)。Google / Microsoft / Okta / Keycloak の大手優勢に整合、REST 原則準拠、Content-Type 将来拡張に中立                                                                                                                                                                  |
| 2026-05-02 | Q10 確定: keystore I/F = **`KeySet { Active(ctx), Verifying(ctx) }`** で設計、M1.1 では `staticKeySet` で内部 1 鍵保持。M2.x の rotation 実装で I/F 変更なし                                                                                                                                                                                     |
| 2026-05-02 | Q12 確定: `token_endpoint_auth_methods_supported` = **`["client_secret_basic"]`** のみ広告。`client_secret_post` は M1.3 で再検討、`private_key_jwt` / `none` は M2.1+                                                                                                                                                                           |
| 2026-05-02 | Q13 確定: issuer 正規化 = **https 必須 (`CORE_ENV=dev` で http 許可)、末尾スラッシュ strip**。ID Token `iss` claim との完全一致担保                                                                                                                                                                                                              |
| 2026-05-02 | Q14 確定: middleware = **M0.2 D1 順序踏襲** (`request_id → access_log → recover → handler`)。OIDC エンドポイント特有の middleware は M1.2+ で D1 内側に追加                                                                                                                                                                                      |
| 2026-05-02 | Q15 確定: Discovery Cache-Control = **`no-cache, must-revalidate` + ETag**、`CORE_OIDC_DISCOVERY_MAX_AGE` env で override 可能 (既定 `0`)。新エンドポイント追加等の即時反映を重視                                                                                                                                                                |
| 2026-05-02 | Q1〜Q15 全件解決完了 (13 件確定 + 2 件削除) → `/requirements-review` フェーズへ移行                                                                                                                                                                                                                                                              |
| 2026-05-02 | レビュー (Codex セカンドオピニオン) で HIGH=2 / MEDIUM=4 / LOW=1 検出 → 全件反映: F-23 (未実装 endpoint = 503 + 機械可読エラー)、F-24 (M1.1 鍵更新非サポート、Pod 全停止 → 起動)、F-1 (Discovery Cache-Control 自己矛盾解消)、F-11 / F-18 (kid と fingerprint 同値明記)、F-19 (規約書必須項目 6 件に拡張)、ユーザーシナリオの dev-keys path 統一 |     |
