package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Server
	Port         string
	Environment  string
	InstanceID   string // Unique per pod for cluster awareness

	// PostgreSQL
	DatabaseURL string

	// Redis (Sentinel mode for HA)
	RedisSentinelAddrs []string
	RedisMasterName    string
	RedisPassword      string

	// Elasticsearch
	ElasticsearchURL string

	// RabbitMQ
	RabbitMQURL string

	// File storage
	UploadDir     string
	MaxUploadSize int64
}

func Load() *Config {
	return &Config{
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),
		InstanceID:  getEnv("HOSTNAME", generateInstanceID()), // K8s sets HOSTNAME to pod name

		DatabaseURL: getEnv("DATABASE_URL", "postgres://app:password@localhost:5432/fileshare?sslmode=disable"),

		RedisSentinelAddrs: []string{getEnv("REDIS_SENTINEL_ADDR", "localhost:26379")},
		RedisMasterName:    getEnv("REDIS_MASTER_NAME", "mymaster"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),

		ElasticsearchURL: getEnv("ELASTICSEARCH_URL", "http://localhost:9200"),

		RabbitMQURL: getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),

		UploadDir:     getEnv("UPLOAD_DIR", "/tmp/uploads"),
		MaxUploadSize: getEnvInt64("MAX_UPLOAD_SIZE", 100*1024*1024), // 100MB
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			return i
		}
	}
	return defaultValue
}

func generateInstanceID() string {
	return "instance-" + strconv.FormatInt(time.Now().UnixNano(), 36)
}
