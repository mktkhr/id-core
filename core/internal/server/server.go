// Package server は HTTP サーバーの構築 (ServeMux + ハンドラ登録) を担う。
//
// M0.1 では /health のみ登録。後続マイルストーンで /authorize, /token, /userinfo,
// /jwks, /.well-known/openid-configuration 等が追加される。
package server

import (
	"fmt"
	"net/http"

	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/health"
)

// New は cfg に従って *http.Server を構築して返す。
//
// ListenAndServe の呼び出しは main 側に委ねる (テスト容易性 / シャットダウン制御の余地確保のため)。
func New(cfg *config.Config) *http.Server {
	mux := http.NewServeMux()

	// Go 1.22+ のメソッド指定パターン: 他メソッドは ServeMux が 405 + Allow ヘッダを返す。
	mux.HandleFunc("GET /health", health.Handler)

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}
}
