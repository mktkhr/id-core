// Package config は core サーバーの実行時設定 (環境変数) の読み込みとバリデーションを担う。
//
// M0.1 では CORE_PORT のみを扱う。後続マイルストーンで DB DSN / ログレベル / OIDC 鍵パス等が追加される想定。
package config

import (
	"fmt"
	"os"
	"strconv"
)

const (
	// DefaultPort は CORE_PORT 未設定時に使用するデフォルトポート。
	DefaultPort = 8080
	// MinPort / MaxPort はリッスンポートとして許容する範囲 (TCP の有効レンジ)。
	MinPort = 1
	MaxPort = 65535
)

// Config は core サーバーの実行時設定を表す。
type Config struct {
	// Port は HTTP サーバーが Listen するポート番号。
	Port int
}

// Load は環境変数から Config を構築して返す。
//
// 取り扱い:
//   - CORE_PORT 未設定または空文字 → DefaultPort (8080)
//   - 数値変換できない / 範囲外 (1〜65535 を逸脱) → error
//
// テスト容易性のため log.Fatal は呼ばず、error として呼び出し元 (main) に返す。
func Load() (*Config, error) {
	port, err := loadPort()
	if err != nil {
		return nil, err
	}
	return &Config{Port: port}, nil
}

func loadPort() (int, error) {
	raw, ok := os.LookupEnv("CORE_PORT")
	// 未設定または明示的に空文字が設定されている場合はデフォルト
	if !ok || raw == "" {
		return DefaultPort, nil
	}

	port, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("CORE_PORT が不正です: %q (%d-%d の整数を指定してください): %w", raw, MinPort, MaxPort, err)
	}

	if port < MinPort || port > MaxPort {
		return 0, fmt.Errorf("CORE_PORT が不正です: %d (許容範囲 %d-%d を逸脱しています)", port, MinPort, MaxPort)
	}

	return port, nil
}
