package config

import (
	"testing"

	config_env "github.com/evolution-foundation/evolution-go/pkg/config/env"
)

// TestLoadEnvIntDefaults cobre os 4 estados do parser de inteiro: ausente,
// válido, não-numérico e <= 0. Inválidos usam o default (nunca Fatal).
func TestLoadEnvIntDefaults(t *testing.T) {
	const key = config_env.POSTGRES_AUTH_MAX_OPEN_CONNS
	const fallback = 10

	cases := []struct {
		name   string
		value  string
		hasEnv bool
		want   int
	}{
		{name: "ausente usa default", hasEnv: false, want: fallback},
		{name: "vazio usa default", hasEnv: true, value: "", want: fallback},
		{name: "válido", hasEnv: true, value: "42", want: 42},
		{name: "não-numérico usa default", hasEnv: true, value: "abc", want: fallback},
		{name: "zero usa default", hasEnv: true, value: "0", want: fallback},
		{name: "negativo usa default", hasEnv: true, value: "-5", want: fallback},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.hasEnv {
				t.Setenv(key, tc.value)
			}
			if got := loadEnvInt(key, fallback); got != tc.want {
				t.Fatalf("loadEnvInt(%s=%q) = %d, want %d", key, tc.value, got, tc.want)
			}
		})
	}
}

// TestLoadEnvDurationDefaults cobre ausente, válido e inválido para as duas
// durações do pool. Inválido loga warn e devolve o default, não derruba.
func TestLoadEnvDurationDefaults(t *testing.T) {
	cases := []struct {
		name     string
		key      string
		fallback string
		value    string
		hasEnv   bool
		want     string
	}{
		{name: "idle ausente", key: config_env.POSTGRES_AUTH_CONN_MAX_IDLE_TIME, fallback: "1m", hasEnv: false, want: "1m"},
		{name: "idle válido", key: config_env.POSTGRES_AUTH_CONN_MAX_IDLE_TIME, fallback: "1m", value: "45s", hasEnv: true, want: "45s"},
		{name: "idle inválido", key: config_env.POSTGRES_AUTH_CONN_MAX_IDLE_TIME, fallback: "1m", value: "um minuto", hasEnv: true, want: "1m"},
		{name: "idle zero", key: config_env.POSTGRES_AUTH_CONN_MAX_IDLE_TIME, fallback: "1m", value: "0s", hasEnv: true, want: "1m"},
		{name: "lifetime ausente", key: config_env.POSTGRES_AUTH_CONN_MAX_LIFETIME, fallback: "30m", hasEnv: false, want: "30m"},
		{name: "lifetime válido", key: config_env.POSTGRES_AUTH_CONN_MAX_LIFETIME, fallback: "30m", value: "2h", hasEnv: true, want: "2h"},
		{name: "lifetime inválido", key: config_env.POSTGRES_AUTH_CONN_MAX_LIFETIME, fallback: "30m", value: "30", hasEnv: true, want: "30m"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.hasEnv {
				t.Setenv(tc.key, tc.value)
			}
			if got := loadEnvDuration(tc.key, tc.fallback); got != tc.want {
				t.Fatalf("loadEnvDuration(%s=%q) = %q, want %q", tc.key, tc.value, got, tc.want)
			}
		})
	}
}

// TestAuthPoolDefaultsAreUsable garante que os defaults do pacote continuam
// respeitando os limites documentados da env.
func TestAuthPoolDefaultsAreUsable(t *testing.T) {
	if defaultAuthMaxOpenConns != 10 {
		t.Fatalf("defaultAuthMaxOpenConns = %d, want 10", defaultAuthMaxOpenConns)
	}
	if defaultAuthMaxIdleConns != 5 {
		t.Fatalf("defaultAuthMaxIdleConns = %d, want 5", defaultAuthMaxIdleConns)
	}
	if defaultAuthConnMaxIdleTime != "1m" {
		t.Fatalf("defaultAuthConnMaxIdleTime = %q, want 1m", defaultAuthConnMaxIdleTime)
	}
	if defaultAuthConnMaxLifetime != "30m" {
		t.Fatalf("defaultAuthConnMaxLifetime = %q, want 30m", defaultAuthConnMaxLifetime)
	}
}
