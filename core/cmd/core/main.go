// Command core は id-core OIDC OP の HTTP サーバーを起動する。
//
// M0.1: /health のみを提供する最小骨格。
// 後続マイルストーンで OIDC エンドポイント群 (authorize / token / userinfo / jwks / discovery) を追加する。
package main

import (
	"log"

	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/logger"
	"github.com/mktkhr/id-core/core/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// M0.2 進行中: 暫定で log.Fatalf。次ステップ (P2 Step 5) で構造化ロガー置換予定。
		log.Fatalf("設定の読み込みに失敗しました: %v", err)
	}

	// M0.2 Step 4: server.New に logger を渡せるよう一時的に Default() で初期化。
	// Step 5 で main 全体を構造化ログ + event_id に書き換える。
	l, lerr := logger.Default()
	if lerr != nil {
		log.Fatalf("ロガー初期化に失敗しました: %v", lerr)
	}

	srv := server.New(cfg, l)

	log.Printf("core サーバーを起動します: addr=%s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("サーバーの実行に失敗しました: %v", err)
	}
}
