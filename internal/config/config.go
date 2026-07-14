package config

import (
	"os"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Port        string `yaml:"port"`
	ServiceName string `yaml:"service_name"`
	LogLevel    string `yaml:"log_level"`

	Auth struct {
		Enabled          bool   `yaml:"enabled"`
		JWTPublicKeyPath string `yaml:"jwt_public_key_path"`
		Issuer           string `yaml:"issuer"`
		Audience         string `yaml:"audience"`
		Algorithm        string `yaml:"algorithm"`
	} `yaml:"auth"`

	RateLimit struct {
		Enabled       bool `yaml:"enabled"`
		RequestsPerSec int `yaml:"requests_per_sec"`
		Burst         int `yaml:"burst"`
		WindowSeconds int `yaml:"window_seconds"`
		FailOpen      bool `yaml:"fail_open"`
	} `yaml:"rate_limit"`

	Cache struct {
		Enabled   bool `yaml:"enabled"`
		TTLSeconds int `yaml:"ttl_seconds"`
		MaxSize   int `yaml:"max_size"`
	} `yaml:"cache"`

	Redis struct {
		Addr     string `yaml:"addr"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
		Timeout  int    `yaml:"timeout_seconds"`
	} `yaml:"redis"`

	CircuitBreaker struct {
		Enabled             bool `yaml:"enabled"`
		MaxRequests         int  `yaml:"max_requests"`
		IntervalSeconds     int  `yaml:"interval_seconds"`
		TimeoutSeconds      int  `yaml:"timeout_seconds"`
		ConsecutiveFailures int  `yaml:"consecutive_failures"`
	} `yaml:"circuit_breaker"`

	Routes []RouteConfig `yaml:"routes"`
}

type RouteConfig struct {
	Path      string   `yaml:"path"`
	Target    string   `yaml:"target"`
	Methods   []string `yaml:"methods"`
	CacheTTL  int      `yaml:"cache_ttl"`
	RateLimit int      `yaml:"rate_limit"`
	Retries   int      `yaml:"retries"`
	Timeout   int      `yaml:"timeout_seconds"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.RateLimit.WindowSeconds == 0 {
		cfg.RateLimit.WindowSeconds = 1
	}
	if cfg.CircuitBreaker.IntervalSeconds == 0 {
		cfg.CircuitBreaker.IntervalSeconds = 10
	}
	if cfg.CircuitBreaker.TimeoutSeconds == 0 {
		cfg.CircuitBreaker.TimeoutSeconds = 30
	}
	if cfg.CircuitBreaker.ConsecutiveFailures == 0 {
		cfg.CircuitBreaker.ConsecutiveFailures = 5
	}

	return &cfg, nil
}
