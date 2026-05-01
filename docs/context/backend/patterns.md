# バックエンド実装パターン (id-core / Go)

> 最終更新: 2026-05-02 (M0.1 反映)

## アーキテクチャパターン

M0.1 段階では最小骨格 (cmd + internal の単純構造)。
M1.x の OIDC エンドポイント実装着手時に **Package by Feature + クリーンアーキテクチャ** を導入する (`backend-architecture` スキル参照)。

## ServeMux ハンドラ登録パターン (Go 1.22+)

メソッド指定パターンを使い、未対応メソッドは `ServeMux` の標準挙動に任せる (405 + `Allow` ヘッダ)。

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /health", health.Handler)
// 他メソッドは自動で 405 + Allow: GET, HEAD
```

## 設定読み込み + main 分離パターン

`config` 層は `error` を返し、`main` で `log.Fatalf` する責務分担。テスト容易性を担保する。

```go
// internal/config/config.go
func Load() (*Config, error) {
    // バリデーション失敗時は error を返す (log.Fatal を直接呼ばない)
}

// cmd/core/main.go
func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("設定の読み込みに失敗しました: %v", err)
    }
    srv := server.New(cfg)
    if err := srv.ListenAndServe(); err != nil {
        log.Fatalf("サーバーの実行に失敗しました: %v", err)
    }
}
```

## httptest ベースのハンドラテストパターン

ServeMux ごと組み立てて `mux.ServeHTTP(rec, req)` を呼ぶ。これによりルーティング (405 / Allow) も検証できる。

```go
func TestHandler_GET_Success(t *testing.T) {
    mux := http.NewServeMux()
    mux.HandleFunc("GET /health", health.Handler)

    req := httptest.NewRequest(http.MethodGet, "/health", nil)
    rec := httptest.NewRecorder()
    mux.ServeHTTP(rec, req)

    res := rec.Result()
    defer res.Body.Close()

    if res.StatusCode != http.StatusOK {
        t.Errorf("status = %d, want %d", res.StatusCode, http.StatusOK)
    }
}
```

## 環境変数を扱うテストパターン (`t.Setenv`)

- `t.Setenv` と `t.Parallel` は**併用不可** (Go の仕様)
- 環境変数を使うテストは直列実行する
- 未設定状態は `t.Setenv("KEY", "")` で空文字を設定し、Load 側で空文字をデフォルト扱いにする (Unsetenv 不可のため)

```go
func TestLoad_Default(t *testing.T) {
    t.Setenv("CORE_PORT", "")
    cfg, err := config.Load()
    // ... デフォルト値検証
}
```

## OIDC クライアント (上流 IdP 委譲) パターン

TBD (M4.x で確定)

## OIDC OP (下流プロダクト向け) パターン

TBD (M1.x で確定)

## 認証セッション管理パターン

TBD

## アカウントリンクパターン

TBD (M4.3 / M5.3 で確定)

## 電話番号 / SNS 認証パターン

TBD (M5.x で確定)

## エラーハンドリング

M0.1 暫定: `fmt.Errorf` で原因を `%w` ラップ。M0.2 で `apperror` パッケージを導入予定。

## DI / 依存注入

M0.1 では `server.New(cfg)` にコンストラクタで設定を渡すだけ。
M1.x で UseCase / Repository を導入する際にコンストラクタ DI を `internal/server/router.go` に集約する。
