// Command core は id-core OIDC OP の HTTP サーバーを起動する。
//
// M0.1: /health のみを提供する最小骨格。
// 後続マイルストーンで OIDC エンドポイント群 (authorize / token / userinfo / jwks / discovery) を追加する。
package main

import (
	"log"

	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// M0.1 暫定: 標準 log.Fatalf で異常終了。
		// 構造化ログへの置換は M0.2 で対応する。
		log.Fatalf("設定の読み込みに失敗しました: %v", err)
	}

	srv := server.New(cfg)

	log.Printf("core サーバーを起動します: addr=%s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("サーバーの実行に失敗しました: %v", err)
	}
}
