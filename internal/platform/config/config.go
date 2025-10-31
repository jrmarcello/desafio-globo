// Pacote config centraliza o carregamento das variáveis de ambiente usadas pelos binários.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config agrega todos os parâmetros necessários para API e worker.
type Config struct {
	HTTPAddress string

	PostgresHost     string
	PostgresPort     string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	PostgresSSLMode  string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	FilaKeyPrefix     string
	ContadorKeyPrefix string

	RateLimitEnabled       bool
	RateLimitMaxActions    int
	RateLimitWindowSeconds int
	RateLimitKeyPrefix     string

	AutoMigrate bool

	WorkerMetricsAddress string
	ConsultaToken        string
}

func Load() (Config, error) {
	// Defaults priorizam execução local; variáveis permitem sobrescrever em Docker/K8s.
	cfg := Config{
		HTTPAddress:            getEnv("HTTP_ADDRESS", ":8080"),
		PostgresHost:           getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:           getEnv("POSTGRES_PORT", "5432"),
		PostgresUser:           getEnv("POSTGRES_USER", "bbb"),
		PostgresPassword:       getEnv("POSTGRES_PASSWORD", "bbb"),
		PostgresDB:             getEnv("POSTGRES_DB", "bbb_votes"),
		PostgresSSLMode:        getEnv("POSTGRES_SSLMODE", "disable"),
		RedisAddr:              getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:          os.Getenv("REDIS_PASSWORD"),
		FilaKeyPrefix:          getEnv("REDIS_QUEUE_PREFIX", "fila:votos"),
		ContadorKeyPrefix:      getEnv("REDIS_COUNTER_PREFIX", "contador"),
		RateLimitEnabled:       getEnv("ANTIFRAUDE_RATE_LIMIT_ENABLED", "true") == "true",
		RateLimitMaxActions:    getEnvAsInt("ANTIFRAUDE_RATE_LIMIT_MAX", 30),
		RateLimitWindowSeconds: getEnvAsInt("ANTIFRAUDE_RATE_LIMIT_WINDOW", 60),
		RateLimitKeyPrefix:     getEnv("ANTIFRAUDE_RATE_LIMIT_PREFIX", "ratelimit"),
		AutoMigrate:            getEnvAsBool("DB_AUTO_MIGRATE", true),
		WorkerMetricsAddress:   getEnv("WORKER_METRICS_ADDRESS", ":9090"),
		ConsultaToken:          os.Getenv("CONSULTA_TOKEN"),
	}

	dbStr := getEnv("REDIS_DB", "0")
	dbInt, err := strconv.Atoi(dbStr)
	if err != nil {
		return Config{}, fmt.Errorf("config: REDIS_DB invalido: %w", err)
	}
	cfg.RedisDB = dbInt

	return cfg, nil
}

func (c Config) PostgresDSN() string {
	// Mantemos o formato DSN compatível com GORM e ferramentas de migração.
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.PostgresUser,
		c.PostgresPassword,
		c.PostgresHost,
		c.PostgresPort,
		c.PostgresDB,
		c.PostgresSSLMode,
	)
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvAsInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return i
}

func getEnvAsBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	switch value {
	case "0", "false", "FALSE", "no", "NO":
		return false
	default:
		return true
	}
}
