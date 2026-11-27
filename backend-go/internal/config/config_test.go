package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromLookup(t *testing.T) {
	t.Parallel()

	validSecret := "0123456789abcdef0123456789abcdef"
	validDBURL := "postgres://postgres@127.0.0.1:5432/truecrime_dev?sslmode=disable"

	tests := []struct {
		name        string
		env         map[string]string
		wantErr     bool
		errContains string
		check       func(t *testing.T, cfg Config)
	}{
		{
			name: "valid required env uses safe defaults",
			env: map[string]string{
				"DATABASE_URL": validDBURL,
				"JWT_SECRET":   validSecret,
				"LISTEN_ADDR":  "127.0.0.1:8080",
			},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				assert.Equal(t, validDBURL, cfg.DatabaseURL)
				assert.Equal(t, validSecret, cfg.JWTSecret)
				assert.Equal(t, "127.0.0.1:8080", cfg.ListenAddr)
				assert.Equal(t, DefaultLogLevel, cfg.LogLevel)
				assert.Equal(t, 60*time.Minute, cfg.JWTTTL)
				assert.Equal(t, DefaultPort, cfg.Port)
				assert.Equal(t, DefaultLoginRatePerMin, cfg.LoginRateLimitPerMin)
			},
		},
		{
			name: "valid custom optional overrides",
			env: map[string]string{
				"DATABASE_URL":             "postgresql://postgres@127.0.0.1:5432/truecrime_dev?sslmode=disable",
				"JWT_SECRET":               validSecret + "extra",
				"LISTEN_ADDR":              "127.0.0.1:9090",
				"LOG_LEVEL":                "debug",
				"JWT_TTL":                  "30m",
				"PORT":                     "9090",
				"APP_VERSION":              "1.0.0-rc1",
				"LOGIN_RATE_LIMIT_PER_MIN": "3",
			},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				assert.Equal(t, "debug", cfg.LogLevel)
				assert.Equal(t, 30*time.Minute, cfg.JWTTTL)
				assert.Equal(t, "9090", cfg.Port)
				assert.Equal(t, "1.0.0-rc1", cfg.Version)
				assert.Equal(t, 3, cfg.LoginRateLimitPerMin)
			},
		},
		{
			name: "missing DATABASE_URL fails fast",
			env: map[string]string{
				"JWT_SECRET":  validSecret,
				"LISTEN_ADDR": "127.0.0.1:8080",
			},
			wantErr:     true,
			errContains: "DATABASE_URL is required",
		},
		{
			name: "missing JWT_SECRET fails fast",
			env: map[string]string{
				"DATABASE_URL": validDBURL,
				"LISTEN_ADDR":  "127.0.0.1:8080",
			},
			wantErr:     true,
			errContains: "JWT_SECRET is required",
		},
		{
			name: "missing LISTEN_ADDR fails fast",
			env: map[string]string{
				"DATABASE_URL": validDBURL,
				"JWT_SECRET":   validSecret,
			},
			wantErr:     true,
			errContains: "LISTEN_ADDR is required",
		},
		{
			name: "short JWT_SECRET under 32 chars fails",
			env: map[string]string{
				"DATABASE_URL": validDBURL,
				"JWT_SECRET":   "too-short-secret-only-27chr",
				"LISTEN_ADDR":  "127.0.0.1:8080",
			},
			wantErr:     true,
			errContains: "JWT_SECRET must be at least 32 characters",
		},
		{
			name: "non-loopback remote DATABASE_URL refused",
			env: map[string]string{
				"DATABASE_URL": "postgres://user:pass@prod.db.internal:5432/truecrime",
				"JWT_SECRET":   validSecret,
				"LISTEN_ADDR":  "127.0.0.1:8080",
			},
			wantErr:     true,
			errContains: "must be 127.0.0.1",
		},
		{
			name: "0.0.0.0 DATABASE_URL refused",
			env: map[string]string{
				"DATABASE_URL": "postgres://postgres@0.0.0.0:5432/truecrime_dev",
				"JWT_SECRET":   validSecret,
				"LISTEN_ADDR":  "127.0.0.1:8080",
			},
			wantErr:     true,
			errContains: "must be 127.0.0.1",
		},
		{
			name: "localhost hostname DATABASE_URL refused (strict 127.0.0.1 guard)",
			env: map[string]string{
				"DATABASE_URL": "postgres://postgres@localhost:5432/truecrime_dev",
				"JWT_SECRET":   validSecret,
				"LISTEN_ADDR":  "127.0.0.1:8080",
			},
			wantErr:     true,
			errContains: "must be 127.0.0.1",
		},
		{
			name: "invalid JWT_TTL fails",
			env: map[string]string{
				"DATABASE_URL": validDBURL,
				"JWT_SECRET":   validSecret,
				"LISTEN_ADDR":  "127.0.0.1:8080",
				"JWT_TTL":      "-5m",
			},
			wantErr:     true,
			errContains: "JWT_TTL",
		},
		{
			name: "invalid LOG_LEVEL fails",
			env: map[string]string{
				"DATABASE_URL": validDBURL,
				"JWT_SECRET":   validSecret,
				"LISTEN_ADDR":  "127.0.0.1:8080",
				"LOG_LEVEL":    "verbose-invalid",
			},
			wantErr:     true,
			errContains: "LOG_LEVEL",
		},
		{
			name: "invalid PORT fails",
			env: map[string]string{
				"DATABASE_URL": validDBURL,
				"JWT_SECRET":   validSecret,
				"LISTEN_ADDR":  "127.0.0.1:8080",
				"PORT":         "99999",
			},
			wantErr:     true,
			errContains: "PORT",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lookup := func(k string) string {
				return tc.env[k]
			}
			cfg, err := LoadFromLookup(lookup)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				return
			}
			require.NoError(t, err)
			if tc.check != nil {
				tc.check(t, cfg)
			}
		})
	}
}
