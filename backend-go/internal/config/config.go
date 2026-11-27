package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

const (
	MinJWTSecretLength     = 32
	DefaultLogLevel        = "info"
	DefaultJWTTTL          = 60 * time.Minute
	DefaultPort            = "8080"
	DefaultVersion         = "0.1.0"
	DefaultLoginRatePerMin = 5
	RequiredLoopbackPGHost = "127.0.0.1"
)

// Config holds immutable runtime configuration validated at startup.
type Config struct {
	DatabaseURL          string
	JWTSecret            string
	ListenAddr           string
	LogLevel             string
	JWTTTL               time.Duration
	Port                 string
	Version              string
	LoginRateLimitPerMin int
}

// Load reads and validates configuration from process environment variables.
func Load() (Config, error) {
	return LoadFromLookup(os.Getenv)
}

// LoadFromLookup reads and validates configuration using the provided lookup function.
// No global state is read or mutated.
func LoadFromLookup(lookup func(string) string) (Config, error) {
	if lookup == nil {
		return Config{}, errors.New("config lookup function is nil")
	}

	var errs []string

	dbURL := strings.TrimSpace(lookup("DATABASE_URL"))
	if dbURL == "" {
		errs = append(errs, "DATABASE_URL is required")
	} else if err := validateLoopbackDatabaseURL(dbURL); err != nil {
		errs = append(errs, err.Error())
	}

	jwtSecret := lookup("JWT_SECRET")
	if strings.TrimSpace(jwtSecret) == "" {
		errs = append(errs, "JWT_SECRET is required")
	} else if len(jwtSecret) < MinJWTSecretLength {
		errs = append(errs, fmt.Sprintf("JWT_SECRET must be at least %d characters (got %d)", MinJWTSecretLength, len(jwtSecret)))
	}

	listenAddr := strings.TrimSpace(lookup("LISTEN_ADDR"))
	if listenAddr == "" {
		errs = append(errs, "LISTEN_ADDR is required")
	}

	logLevel := strings.TrimSpace(lookup("LOG_LEVEL"))
	if logLevel == "" {
		logLevel = DefaultLogLevel
	} else {
		logLevel = strings.ToLower(logLevel)
		if _, err := zerolog.ParseLevel(logLevel); err != nil {
			errs = append(errs, fmt.Sprintf("LOG_LEVEL %q is invalid: %v", logLevel, err))
		}
	}

	jwtTTL := DefaultJWTTTL
	if rawTTL := strings.TrimSpace(lookup("JWT_TTL")); rawTTL != "" {
		parsedTTL, err := time.ParseDuration(rawTTL)
		if err != nil || parsedTTL <= 0 {
			errs = append(errs, fmt.Sprintf("JWT_TTL %q must be a positive duration", rawTTL))
		} else {
			jwtTTL = parsedTTL
		}
	}

	port := strings.TrimSpace(lookup("PORT"))
	if port == "" {
		port = DefaultPort
	} else {
		pNum, err := strconv.Atoi(port)
		if err != nil || pNum < 1 || pNum > 65535 {
			errs = append(errs, fmt.Sprintf("PORT %q must be an integer between 1 and 65535", port))
		}
	}

	version := strings.TrimSpace(lookup("APP_VERSION"))
	if version == "" {
		version = DefaultVersion
	}

	loginRateLimit := DefaultLoginRatePerMin
	if rawLimit := strings.TrimSpace(lookup("LOGIN_RATE_LIMIT_PER_MIN")); rawLimit != "" {
		n, err := strconv.Atoi(rawLimit)
		if err != nil || n < 1 {
			errs = append(errs, fmt.Sprintf("LOGIN_RATE_LIMIT_PER_MIN %q must be a positive integer", rawLimit))
		} else {
			loginRateLimit = n
		}
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %s", strings.Join(errs, "; "))
	}

	return Config{
		DatabaseURL:          dbURL,
		JWTSecret:            jwtSecret,
		ListenAddr:           listenAddr,
		LogLevel:             logLevel,
		JWTTTL:               jwtTTL,
		Port:                 port,
		Version:              version,
		LoginRateLimitPerMin: loginRateLimit,
	}, nil
}

func validateLoopbackDatabaseURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("DATABASE_URL is not a valid URL: %w", err)
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return fmt.Errorf("DATABASE_URL scheme %q must be postgres or postgresql", parsed.Scheme)
	}
	host := parsed.Hostname()
	if host != RequiredLoopbackPGHost {
		return fmt.Errorf("refusing to start: DATABASE_URL host %q must be %s (local loopback guard)", host, RequiredLoopbackPGHost)
	}
	return nil
}
