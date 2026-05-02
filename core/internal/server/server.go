// Package server は HTTP サーバーの構築 (ServeMux + ハンドラ登録 + middleware チェーン) を担う。
//
// M0.2 で middleware チェーン (request_id / access_log / recover) が組み込まれた。
// M0.3 で /health/live (DB 非依存) + /health/ready (DB Ping) を追加し、
// 既存 /health は外形互換のまま維持する。
// 後続マイルストーンで /authorize, /token, /userinfo, /jwks, /.well-known/openid-configuration
// 等が追加される。
package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/health"
	"github.com/mktkhr/id-core/core/internal/logger"
	"github.com/mktkhr/id-core/core/internal/middleware"
)

// readHeaderTimeout はリクエストヘッダ読み取り全体のタイムアウト。
// Slowloris 攻撃 (CWE-400) 対策。後続マイルストーンで他のタイムアウト
// (ReadTimeout / WriteTimeout / IdleTimeout) も追加する。
const readHeaderTimeout = 10 * time.Second

// New は cfg / logger / pgxpool に従って *http.Server を構築して返す。
//
// M0.3 でシグネチャに pool を追加: /health/ready が pool.Ping で readiness を判定するため。
//
// 注意: 本関数は context.Context を第 1 引数に取らない。
// 設計書 #21 の F-18 適用範囲は `internal/db/` / `internal/dbmigrate/` /
// `internal/testutil/dbtest/` の DB 公開 API に限定されており、HTTP サーバ構築
// (server.New) およびリクエストハンドラ生成 (health.NewLiveHandler / NewReadyHandler)
// は対象外。各リクエストの context は r.Context() からハンドラ内で取得する慣習に従う。
//
// middleware チェーン (D1 順序、外側から内側へ):
//
//	request_id → access_log → recover → handler
//
// この順序により:
//   - 全ログレコードに request_id が付く (panic 含む)
//   - access_log は recover が変換した最終 status (500 等) を観測できる
//
// ListenAndServe の呼び出しは main 側に委ねる
// (テスト容易性 / シャットダウン制御の余地確保のため)。
//
// l == nil または pool == nil の場合は契約違反として panic する。
func New(cfg *config.Config, l *logger.Logger, pool *pgxpool.Pool) *http.Server {
	if l == nil {
		panic("server.New: logger must not be nil (M0.2 シングルポイント設計の契約)")
	}
	if pool == nil {
		panic("server.New: pool must not be nil (M0.3: /health/ready が pool.Ping を要求)")
	}
	mux := http.NewServeMux()

	// Go 1.22+ のメソッド指定パターン: 他メソッドは ServeMux が 405 + Allow ヘッダを返す。
	mux.HandleFunc("GET /health", health.NewHandler(l))                    // M0.1 後方互換
	mux.HandleFunc("GET /health/live", health.NewLiveHandler(l))           // M0.3 Q6
	mux.HandleFunc("GET /health/ready", health.NewReadyHandler(pool, l))   // M0.3 Q6

	// 内側から外側へ wrap (実行は外側から): request_id が最外側、handler が最内側。
	var wrapped http.Handler = mux
	wrapped = middleware.Recover(l, wrapped)
	wrapped = middleware.AccessLog(l, wrapped)
	wrapped = middleware.RequestID(wrapped)

	return &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           wrapped,
		ReadHeaderTimeout: readHeaderTimeout,
	}
}
