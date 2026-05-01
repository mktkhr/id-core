package config_test

import (
	"strings"
	"testing"

	"github.com/mktkhr/id-core/core/internal/config"
)

// 注: t.Setenv は t.Parallel と併用不可なため、各サブテストは直列実行する。
//     CORE_PORT 環境変数を扱う性質上、テスト間の干渉を防ぐ目的でも直列が適切。

func TestLoad_Success(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		setEnv   bool
		wantPort int
	}{
		// T-5: CORE_PORT 未設定 → デフォルト 8080
		{name: "T-5: CORE_PORT 未設定 → デフォルト 8080", setEnv: false, wantPort: 8080},
		// T-6: CORE_PORT=9000 → Port=9000
		{name: "T-6: CORE_PORT=9000 → Port=9000", envValue: "9000", setEnv: true, wantPort: 9000},
		// T-10: 境界値 (CORE_PORT=1)
		{name: "T-10: CORE_PORT=1 (下限) → 正常", envValue: "1", setEnv: true, wantPort: 1},
		// T-10: 境界値 (CORE_PORT=65535)
		{name: "T-10: CORE_PORT=65535 (上限) → 正常", envValue: "65535", setEnv: true, wantPort: 65535},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv("CORE_PORT", tt.envValue)
			} else {
				// 未設定状態を作るため空文字を設定 (Load 側で空文字をデフォルト扱い)
				t.Setenv("CORE_PORT", "")
			}

			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("config.Load() でエラーが返ってはいけない: %v", err)
			}
			if cfg == nil {
				t.Fatalf("config.Load() が nil を返した")
			}
			if cfg.Port != tt.wantPort {
				t.Errorf("cfg.Port = %d, want %d", cfg.Port, tt.wantPort)
			}
		})
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
	}{
		// T-7: 非数値
		{name: "T-7: CORE_PORT=abc → エラー", envValue: "abc"},
		// T-8: 0 (下限未満)
		{name: "T-8: CORE_PORT=0 → エラー", envValue: "0"},
		// T-9: 65536 (上限超)
		{name: "T-9: CORE_PORT=65536 → エラー", envValue: "65536"},
		// 追加: 負数
		{name: "CORE_PORT=-1 → エラー", envValue: "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CORE_PORT", tt.envValue)

			cfg, err := config.Load()
			if err == nil {
				t.Fatalf("config.Load() がエラーを返さなかった: cfg=%+v", cfg)
			}
			// 設計書: メッセージに「CORE_PORT が不正」を含む
			if !strings.Contains(err.Error(), "CORE_PORT が不正") {
				t.Errorf("エラーメッセージに 'CORE_PORT が不正' を含むべき: got %q", err.Error())
			}
		})
	}
}
