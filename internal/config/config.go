package config

import (
    "fmt"
    "os"
)
//getEnv is a small helper function in your config package whose job is(Read an environment variable, If it is empty or not set → return a default value (fallback))
type Config struct {
    DBHost      string
    DBPort      string
    DBUser      string
    DBPassword  string
    DBName      string
    DBSSLMode   string
    HTTPPort    string
    Seed        bool   // seed demo data when the database is empty
    CORSOrigins string // value for Access-Control-Allow-Origin; empty disables CORS
}

func Load() *Config {
    return &Config{
        DBHost:      getEnv("DB_HOST", "localhost"),//getEnv("DB_HOST", "localhost") — read DB_HOST from environment; if empty, use "localhost".
        DBPort:      getEnv("DB_PORT", "5432"),
        DBUser:      getEnv("DB_USER", "postgres"),
        DBPassword:  getEnv("DB_PASSWORD", "postgres"),
        DBName:      getEnv("DB_NAME", "studentdb"),
        DBSSLMode:   getEnv("DB_SSLMODE", "disable"),
        HTTPPort:    getEnv("HTTP_PORT", "8080"),
        Seed:        getEnv("SEED_DEMO_DATA", "true") == "true",
        CORSOrigins: getEnv("CORS_ORIGINS", "*"),
    }
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}

func (c *Config) PostgresDSN() string {
    return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
        c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode)
}
