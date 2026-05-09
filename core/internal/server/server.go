// Package server は HTTP サーバーの構築 (ServeMux + ハンドラ登録 + middleware チェーン) を担う。
//
// M0.2 で middleware チェーン (request_id / access_log / recover) が組み込まれた。
// M0.3 で /health/live (DB 非依存) + /health/ready (DB Ping) を追加し、
// 既存 /health は外形互換のまま維持する。
// M1.1 で OIDC OP の公開エンドポイント群を追加 (設計 #32):
//   - GET /.well-known/openid-configuration  (Discovery、F-1〜F-4)
//   - GET /jwks                              (JWKS、F-5/F-6)
//   - GET /authorize / POST /token / GET /userinfo  (notimpl 503 stub、F-23、M1.2-1.4 で本実装)
//
// すべて公開エンドポイント (認証不要、F-16) で middleware D1 順序 (request_id → access_log → recover)
// を共有する。
package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/health"
	"github.com/mktkhr/id-core/core/internal/keystore"
	"github.com/mktkhr/id-core/core/internal/logger"
	"github.com/mktkhr/id-core/core/internal/middleware"
	"github.com/mktkhr/id-core/core/internal/oidc/discovery"
	"github.com/mktkhr/id-core/core/internal/oidc/jwks"
	"github.com/mktkhr/id-core/core/internal/oidc/notimpl"
)

// readHeaderTimeout はリクエストヘッダ読み取り全体のタイムアウト。
// Slowloris 攻撃 (CWE-400) 対策。後続マイルストーンで他のタイムアウト
// (ReadTimeout / WriteTimeout / IdleTimeout) も追加する。
const readHeaderTimeout = 10 * time.Second

// New は cfg / logger / pgxpool / keystore に従って *http.Server を構築して返す。
//
// シグネチャの変遷:
//   - M0.2: New(cfg, l)
//   - M0.3: New(cfg, l, pool)  ← /health/ready が pool.Ping で readiness 判定
//   - M1.1: New(cfg, l, pool, ks) (*http.Server, error)  ← OIDC route 追加 + Discovery/JWKS Handler 構築失敗を error で伝播
//
// 注意: 本関数は context.Context を第 1 引数に取らない。
// 設計書 #21 の F-18 適用範囲は `internal/db/` / `internal/dbmigrate/` /
// `internal/testutil/dbtest/` の DB 公開 API に限定されており、HTTP サーバ構築
// (server.New) およびリクエストハンドラ生成は対象外。各リクエストの context は
// r.Context() からハンドラ内で取得する慣習に従う。
//
// middleware チェーン (D1 順序、外側から内側へ):
//
//	request_id → access_log → recover → handler
//
// この順序により:
//   - 全ログレコードに request_id が付く (panic 含む)
//   - access_log は recover が変換した最終 status (500 等) を観測できる
//   - 全 OIDC route も含めて全 endpoint が同一 middleware を通過する
//
// ListenAndServe の呼び出しは main 側に委ねる
// (テスト容易性 / シャットダウン制御の余地確保のため)。
//
// l == nil または pool == nil または ks == nil の場合は契約違反として panic する。
//
// 戻り値の error は Discovery / JWKS handler 構築失敗時 (Marshal 失敗、KeySet が壊れている等)。
// main.go 側で logger.Error + os.Exit(1) する責務分担。
func New(cfg *config.Config, l *logger.Logger, pool *pgxpool.Pool, ks keystore.KeySet) (*http.Server, error) {
	if l == nil {
		panic("server.New: logger must not be nil (M0.2 シングルポイント設計の契約)")
	}
	if pool == nil {
		panic("server.New: pool must not be nil (M0.3: /health/ready が pool.Ping を要求)")
	}
	if ks == nil {
		panic("server.New: keystore.KeySet must not be nil (M1.1: JWKS handler が Verifying を呼ぶ)")
	}

	discoveryH, err := discovery.New(cfg.OIDC)
	if err != nil {
		return nil, fmt.Errorf("server.New: discovery.New に失敗しました: %w", err)
	}
	jwksH, err := jwks.New(ks, cfg.OIDC.JWKSMaxAge)
	if err != nil {
		return nil, fmt.Errorf("server.New: jwks.New に失敗しました: %w", err)
	}

	mux := http.NewServeMux()

	// Go 1.22+ のメソッド指定パターン: 他メソッドは ServeMux が 405 + Allow ヘッダを返す。
	mux.HandleFunc("GET /health", health.NewHandler(l))                  // M0.1 後方互換
	mux.HandleFunc("GET /health/live", health.NewLiveHandler(l))         // M0.3 Q6
	mux.HandleFunc("GET /health/ready", health.NewReadyHandler(pool, l)) // M0.3 Q6

	// OIDC 公開エンドポイント (M1.1、認証不要 / F-16)
	mux.Handle("GET /.well-known/openid-configuration", discoveryH) // F-1〜F-4
	mux.Handle("GET /jwks", jwksH)                                  // F-5/F-6 (Q9: 拡張子なし)

	// 未実装 endpoint stub (M1.2-1.4 で本実装、F-23)。
	// Discovery メタデータで広告するため必ず route 登録するが、現状は 503 + 機械可読 JSON を返す。
	mux.HandleFunc("GET /authorize", notimpl.Handler("M1.2"))
	mux.HandleFunc("POST /token", notimpl.Handler("M1.3"))
	mux.HandleFunc("GET /userinfo", notimpl.Handler("M1.4"))

	// 内側から外側へ wrap (実行は外側から): request_id が最外側、handler が最内側。
	var wrapped http.Handler = mux
	wrapped = middleware.Recover(l, wrapped)
	wrapped = middleware.AccessLog(l, wrapped)
	wrapped = middleware.RequestID(wrapped)

	return &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           wrapped,
		ReadHeaderTimeout: readHeaderTimeout,
	}, nil
}
