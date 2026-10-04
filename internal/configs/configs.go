package configs

import (
	"errors"
	"os"
	"time"

	"github.com/baobei23/goapp/cmd/server/http"
	"github.com/baobei23/goapp/internal/pkg/jwt"
	"github.com/baobei23/goapp/internal/pkg/postgres"
)

type env string

func (e env) String() string {
	return string(e)
}

const (
	EnvLocal      env = "local"
	EnvTest       env = "test"
	EnvStaging    env = "staging"
	EnvProduction env = "production"
)

// Configs struct handles all dependencies required for handling configurations
type Configs struct {
	Environment   env
	AppName       string
	AppVersion    string
	EnableTracing bool
	EnableMetrics bool
}

// HTTP returns the configuration required for HTTP package
func (cfg *Configs) HTTP() (*http.Config, error) {
	return &http.Config{
		EnableAccessLog: (cfg.Environment == EnvLocal) || (cfg.Environment == EnvTest),
		Port:            8080,
		ReadTimeout:     time.Second * 5,
		WriteTimeout:    time.Second * 5,
		EnableTracing:   cfg.EnableTracing,
	}, nil
}

func (cfg *Configs) Postgres() *postgres.Config {
	return &postgres.Config{
		Host:    os.Getenv("POSTGRES_HOST"),
		Port:    os.Getenv("POSTGRES_PORT"),
		Driver:  "postgres",
		SSLMode: os.Getenv("POSTGRES_SSLMODE"),

		StoreName: os.Getenv("POSTGRES_STORENAME"),
		Username:  os.Getenv("POSTGRES_USERNAME"),
		Password:  os.Getenv("POSTGRES_PASSWORD"),

		ConnPoolSize:  24,
		IdleTimeout:   time.Minute,
		DialTimeout:   time.Second * 3,
		EnableTracing: cfg.EnableTracing,
	}
}

func (cfg *Configs) JWT() *jwt.TokenManager {
	return &jwt.TokenManager{
		SecretKey:     os.Getenv("JWT_SECRET"),
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 30 * 24 * time.Hour,
	}
}

// Tambahkan method Validate() pada struct Configs
func (cfg *Configs) Validate() error {
	if os.Getenv("JWT_SECRET") == "" {
		return errors.New("configuration error: JWT_SECRET environment variable is required")
	}
	if len(os.Getenv("JWT_SECRET")) < 32 {
		return errors.New("configuration error: JWT_SECRET must be at least 32 characters for security")
	}
	if os.Getenv("POSTGRES_HOST") == "" {
		return errors.New("configuration error: POSTGRES_HOST is required")
	}
	return nil
}

func loadEnv() env {
	switch env(os.Getenv("ENV")) {
	case EnvLocal:
		return EnvLocal
	case EnvTest:
		return EnvTest
	case EnvStaging:
		return EnvProduction
	case EnvProduction:
		return EnvProduction
	default:
		return EnvLocal
	}
}

// New returns an instance of Config with all the required dependencies initialized
func New() (*Configs, error) {
	return &Configs{
		Environment:   loadEnv(),
		AppName:       os.Getenv("APP_NAME"),
		AppVersion:    os.Getenv("APP_VERSION"),
		EnableTracing: os.Getenv("ENABLE_TRACING") != "false",
		EnableMetrics: os.Getenv("ENABLE_METRICS") != "false",
	}, nil
}
