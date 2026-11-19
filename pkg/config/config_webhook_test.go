package config

import (
	"testing"

	config_env "github.com/evolution-foundation/evolution-go/pkg/config/env"
)

// TestWebhookSkipVideoDefaultsFalse garante o contrato da env `WEBHOOK_SKIP_VIDEO`:
// ausente/vazia/lixo mantêm o comportamento histórico (false = baixa vídeo e
// emite base64); só "true" liga o skip.
func TestWebhookSkipVideoDefaultsFalse(t *testing.T) {
	cases := []struct {
		name   string
		value  string
		hasEnv bool
		want   bool
	}{
		{name: "ausente preserva comportamento atual", hasEnv: false, want: false},
		{name: "vazia preserva comportamento atual", hasEnv: true, value: "", want: false},
		{name: "false", hasEnv: true, value: "false", want: false},
		{name: "true liga o skip", hasEnv: true, value: "true", want: true},
		{name: "TRUE é aceito", hasEnv: true, value: "TRUE", want: true},
		{name: "valor inválido cai no default", hasEnv: true, value: "sim!", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.hasEnv {
				t.Setenv(config_env.WEBHOOK_SKIP_VIDEO, tc.value)
			}
			if got := loadEnvBoolDefaultFalse(config_env.WEBHOOK_SKIP_VIDEO); got != tc.want {
				t.Fatalf("WEBHOOK_SKIP_VIDEO=%q (hasEnv=%v) = %v, want %v", tc.value, tc.hasEnv, got, tc.want)
			}
		})
	}
}
