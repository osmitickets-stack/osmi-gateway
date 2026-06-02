package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	HTTPPort       string
	GRPCServerAddr string
	Environment    string
	LogLevel       string
	ReadTimeout    int
	WriteTimeout   int
	JWTSecret      string
	RedisURL       string
	RedisPassword  string
	RedisDB        int
}

func Load() *Config {
	return &Config{
		HTTPPort:       getEnv("HTTP_PORT", "8083"),
		GRPCServerAddr: getEnv("GRPC_SERVER_ADDR", "localhost:50051"),
		Environment:    getEnv("ENVIRONMENT", "development"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		ReadTimeout:    getEnvAsInt("READ_TIMEOUT", 15),
		WriteTimeout:   getEnvAsInt("WRITE_TIMEOUT", 15),
		JWTSecret:      getEnv("JWT_SECRET_KEY", ""),
		RedisURL:       getEnv("REDIS_URL", "localhost:6379"),
		RedisPassword:  getEnv("REDIS_PASSWORD", ""),
		RedisDB:        getEnvAsInt("REDIS_DB", 0),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
		log.Printf("Warning: invalid integer for %s, using default", key)
	}
	return defaultValue
}
