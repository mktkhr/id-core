package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/logger"
)

// Open は cfg から pgxpool.Pool を生成し、初回 Ping を実行して接続性を検証する。
//
// 動作:
//  1. BuildDSN で組み立てた DSN を pgxpool.ParseConfig に渡す
//  2. cfg.Database のプール設定 (MaxConns / MinConns / Lifetime / IdleTime / HealthCheckPeriod) を反映
//  3. pgxpool.NewWithConfig で Pool 作成
//  4. ctx を伝播した Ping で初回接続を確認
//  5. 失敗時はエラーログを出力し、Pool を Close してから error 返却
//
// ログには SafeRepr の値のみを利用し、DSN フルダンプ・パスワードを出してはならない (F-10)。
// 第 1 引数は context.Context (F-18) で、Open / Ping のキャンセル伝播に利用する。
func Open(ctx context.Context, cfg *config.DatabaseConfig, l *logger.Logger) (*pgxpool.Pool, error) {
	if cfg == nil {
		return nil, errors.New("db.Open: cfg は nil にできません")
	}
	if l == nil {
		return nil, errors.New("db.Open: logger は nil にできません")
	}

	dsn := BuildDSN(cfg)
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		l.Error(ctx, "DB DSN の parse に失敗しました", err, "params", SafeRepr(cfg))
		return nil, fmt.Errorf("db.Open: pgxpool.ParseConfig: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		l.Error(ctx, "DB 接続プールの生成に失敗しました", err, "params", SafeRepr(cfg))
		return nil, fmt.Errorf("db.Open: pgxpool.NewWithConfig: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		l.Error(ctx, "DB 初回 Ping に失敗しました", err, "params", SafeRepr(cfg))
		pool.Close()
		return nil, fmt.Errorf("db.Open: pool.Ping: %w", err)
	}

	return pool, nil
}
