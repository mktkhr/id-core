// Command core は id-core OIDC OP の HTTP サーバーを起動する。
//
// M0.2: 構造化ログ + request_id middleware を組み込んだ最小骨格。
// 後続マイルストーンで OIDC エンドポイント群 (authorize / token / userinfo / jwks /
// discovery) を追加する。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/logger"
	"github.com/mktkhr/id-core/core/internal/server"
)

// exitCode は run() の終了コード。標準 Unix 慣例に従い 0=成功 / 1=失敗。
const (
	exitOK    = 0
	exitError = 1
)

func main() {
	os.Exit(run(os.Stderr))
}

// run は main 本体を testable に切り出した関数。
//
// 起動失敗時のロガー初期化前のログは fallback (引数 stderr) に直接書く。
// 通常時のログは構造化ロガー経由で stdout に出る。
//
// 終了コードを int で返すことで、テスト側から異常系の直接実行と検証が可能。
func run(stderr *os.File) int {
	cfg, l, eventID, err := bootstrap()
	if err != nil {
		// bootstrap 失敗時は構造化ロガーが使えない可能性があるため、
		// 引数 stderr に最終フォールバック行を出す。
		fmt.Fprintf(stderr, "起動準備に失敗しました: %v\n", err)
		return exitError
	}

	ctx := logger.WithEventID(context.Background(), eventID)

	srv := server.New(cfg, l)

	emitStartupLog(l, ctx, srv.Addr)

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		l.Error(ctx, "サーバーの実行に失敗しました", err)
		return exitError
	}
	return exitOK
}

// emitStartupLog は起動 INFO ログを構造化形式で出力する (F-4 / F-14 / Q3)。
//
// 必須フィールド:
//   - msg: "core サーバーを起動します" (日本語)
//   - addr: 待ち受けアドレス (例: ":8080")
//   - event_id: ctx に WithEventID 済みの UUID v7 が logger により自動付与される
//
// event_id は ctx 単一ソースに統一する (引数二重指定で値が食い違うバグを避ける)。
// run() から分離することで、テストで任意の logger を注入して JSON スキーマを検証できる。
func emitStartupLog(l *logger.Logger, ctx context.Context, addr string) {
	l.Info(ctx, "core サーバーを起動します",
		slog.String("addr", addr),
	)
}

// bootstrap は起動前の設定読み込み・ロガー初期化・event_id 発番をまとめて行う。
// 各失敗を error として返し、run() で終了コード判定する責務分離。
func bootstrap() (*config.Config, *logger.Logger, string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, "", fmt.Errorf("設定の読み込みに失敗しました: %w", err)
	}

	l, err := logger.Default()
	if err != nil {
		return nil, nil, "", fmt.Errorf("ロガーの初期化に失敗しました: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, nil, "", fmt.Errorf("event_id (UUID v7) の生成に失敗しました: %w", err)
	}

	return cfg, l, id.String(), nil
}
