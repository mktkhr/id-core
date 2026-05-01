---
name: backend-logging
description: >-
  バックエンドのログ規約ガイド (構造化ログ / 重複ログ禁止 / Domain 層ログ禁止)。"backend-logging", "ログ実装",
  "ログ規約" 等で発動。
---
# Backend Logging Guide

## 原則

1. **構造化ログ** (Go: `log/slog` または同等。Kotlin: SLF4J + Logback / Logstash JSON encoder)
2. **同一リクエストに同一 `request_id` を付与** (ミドルウェアで自動付与)
3. **同一エラーを複数層でログしない** (責任層で 1 度だけ)
4. **Domain 層ではログを出力しない** (副作用を持たない)
5. **すべてのログメッセージは日本語で記載する**
6. **機密情報 (トークン全文、パスワード、PII) はログに出力しない**

## ログレベル

| レベル | 用途 | 本番 |
|---|---|---|
| DEBUG | 開発・調査 (クエリ内容、I/O 時間) | 無効 |
| INFO | 業務イベント (リクエスト開始/終了、成功) | 有効 |
| WARN | 想定外だが処理継続可能 (4xx エラー) | 有効 |
| ERROR | 処理失敗・異常 (DB 障害、予期しないエラー) | 有効 |

## 層ごとの出力

### Presentation 層

リクエスト/レスポンスのライフサイクル、最終的なエラーレスポンス。

```go
// Go (例)
requestLogger := logger.NewRequestLoggerFromGin(c)
requestLogger.Info("処理開始")
requestLogger.Warn("バリデーションエラー", "error", err)
requestLogger.Error("処理失敗", "error", err)
```

### UseCase 層

業務イベント (DEBUG/INFO のみ)。**ERROR は出さず Presentation までエスカレーション**。

```go
logger.InfoWithContext(ctx, "備品作成開始")
logger.DebugWithContext(ctx, "入力値", "user_id", userID)
```

### Infrastructure 層

DB / 外部 API / キャッシュ操作の DEBUG・ERROR。

```go
logger.DebugWithContext(ctx, "クエリ開始", "table", "items")
logger.LogDatabaseError(ctx, "CREATE", "items", err, "name", itemName)
```

### Domain 層

**ログ出さない**。エラーは値として返す。

## メソッドトレース (任意)

```go
defer logger.TraceMethodAuto(ctx, param)()
```

記録: `request_id` / `goroutine_id` / `method` / `phase` / `duration_ms`

## エラーログ分類 (推奨ヘルパー)

```go
logger.LogDatabaseError(ctx, "CREATE", "items", err, "name", itemName)
logger.LogBusinessError(ctx, "重複チェック", err, "name", itemName)
logger.LogValidationError(ctx, "name", itemName, "required")
```

## 重複ログ禁止

```
✅ Repository 層でのみログ → UseCase は return err → Presentation は HTTP レスポンスのみ
❌ 3 層すべてでログ出力
```

## ログ解析 (jq)

JSON ログを前提に、jq で絞り込み解析する。

```bash
# 4xx/5xx を抽出
jq 'select(.status >= 400) | {request_id, status, path, error}' app.log

# 特定 request_id のトレース
jq 'select(.request_id == "xxx")' app.log

# DB エラーの分類
jq 'select(.error_type == "database") | {table, operation, db_error_code}' app.log
```

## OIDC 特有の注意

- ID トークン / アクセストークン / リフレッシュトークン**全文をログに出さない** (jti / sub / 先頭数文字 のみ)
- `client_secret`, `code`, `code_verifier` の値を出さない
- consent / authorize / token エンドポイントは `request_id` + `client_id` + `subject` で追跡可能にする

## 保管 (本番想定)

- ローカル: 標準出力
- 本番: 集約基盤 (CloudWatch / ELK / Loki 等)
- 保持: ERROR 30 日 / アクセス 7 日 (要件に応じて調整)
