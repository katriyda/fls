package config

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port               int           `json:"port"`
	DataDir            string        `json:"data_dir"`
	TokenLength        int           `json:"token_length"`
	MaxUploadSize      int64         `json:"max_upload_size"`
	DefaultExpiry      time.Duration `json:"default_expiry"`
	SessionTimeout     time.Duration `json:"session_timeout"`
	LogRetentionDays   int           `json:"log_retention_days"`
	RateLimitPerMinute int           `json:"rate_limit_per_minute"`
	db                 *sql.DB
}

func Defaults() *Config {
	return &Config{
		Port:               8080,
		DataDir:            "./data",
		TokenLength:        8,
		MaxUploadSize:      10 * 1024 * 1024 * 1024, // 10GB
		DefaultExpiry:      7 * 24 * time.Hour,       // 7 days
		SessionTimeout:     24 * time.Hour,
		LogRetentionDays:   90,
		RateLimitPerMinute: 60,
	}
}

func New(db *sql.DB) *Config {
	c := Defaults()
	c.db = db
	c.Load()
	return c
}

func (c *Config) Load() {
	if c.db == nil {
		return
	}

	rows, err := c.db.Query("SELECT key, value FROM config")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		c.apply(key, value)
	}
}

func (c *Config) Save() error {
	if c.db == nil {
		return fmt.Errorf("database not connected")
	}

	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	pairs := map[string]string{
		"port":                  strconv.Itoa(c.Port),
		"data_dir":              c.DataDir,
		"token_length":          strconv.Itoa(c.TokenLength),
		"max_upload_size":       strconv.FormatInt(c.MaxUploadSize, 10),
		"default_expiry":        c.DefaultExpiry.String(),
		"session_timeout":       c.SessionTimeout.String(),
		"log_retention_days":    strconv.Itoa(c.LogRetentionDays),
		"rate_limit_per_minute": strconv.Itoa(c.RateLimitPerMinute),
	}

	stmt, err := tx.Prepare("INSERT OR REPLACE INTO config (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)")
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for key, value := range pairs {
		if _, err := stmt.Exec(key, value); err != nil {
			return fmt.Errorf("save config %s: %w", key, err)
		}
	}

	return tx.Commit()
}

func (c *Config) ApplyOverrides(port int, dataDir string) {
	if port != 0 {
		c.Port = port
	}
	if dataDir != "" {
		c.DataDir = dataDir
	}
}

func (c *Config) apply(key, value string) {
	switch key {
	case "port":
		if v, err := strconv.Atoi(value); err == nil {
			c.Port = v
		}
	case "data_dir":
		c.DataDir = value
	case "token_length":
		if v, err := strconv.Atoi(value); err == nil {
			c.TokenLength = v
		}
	case "max_upload_size":
		if v, err := strconv.ParseInt(value, 10, 64); err == nil {
			c.MaxUploadSize = v
		}
	case "default_expiry":
		if v, err := time.ParseDuration(value); err == nil {
			c.DefaultExpiry = v
		}
	case "session_timeout":
		if v, err := time.ParseDuration(value); err == nil {
			c.SessionTimeout = v
		}
	case "log_retention_days":
		if v, err := strconv.Atoi(value); err == nil {
			c.LogRetentionDays = v
		}
	case "rate_limit_per_minute":
		if v, err := strconv.Atoi(value); err == nil {
			c.RateLimitPerMinute = v
		}
	}
}

func (c *Config) EnvOverrides() {
	if v := os.Getenv("FLS_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.Port = p
		}
	}
	if v := os.Getenv("FLS_DATA_DIR"); v != "" {
		c.DataDir = v
	}
	if v := os.Getenv("FLS_TOKEN_LENGTH"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.TokenLength = p
		}
	}
	if v := os.Getenv("FLS_MAX_UPLOAD_SIZE"); v != "" {
		if p, err := strconv.ParseInt(v, 10, 64); err == nil {
			c.MaxUploadSize = p
		}
	}
	if v := os.Getenv("FLS_DEFAULT_EXPIRY"); v != "" {
		if p, err := time.ParseDuration(v); err == nil {
			c.DefaultExpiry = p
		}
	}
	if v := os.Getenv("FLS_SESSION_TIMEOUT"); v != "" {
		if p, err := time.ParseDuration(v); err == nil {
			c.SessionTimeout = p
		}
	}
	if v := os.Getenv("FLS_LOG_RETENTION_DAYS"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.LogRetentionDays = p
		}
	}
	if v := os.Getenv("FLS_RATE_LIMIT_PER_MINUTE"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.RateLimitPerMinute = p
		}
	}
}
