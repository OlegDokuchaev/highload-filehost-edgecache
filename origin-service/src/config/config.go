package config

import (
	"fmt"
	"os"

	yaml "gopkg.in/yaml.v3"
)

type Config struct {
	Logging LoggingConfig `yaml:"logging"`
	App     AppConfig     `yaml:"app"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`  // debug, info, warn, error
	Format string `yaml:"format"` // json, text
	Output string `yaml:"output"` // stdout, stderr, file
	File   string `yaml:"file"`   // путь к файлу лога (output=file)
}

type AppConfig struct {
	Port              string `yaml:"port"`
	MinioEndpoint     string `yaml:"minio_endpoint"`
	MinioAccessKey    string `yaml:"minio_access_key"`
	MinioSecretKey    string `yaml:"minio_secret_key"`
	MinioBucket       string `yaml:"minio_bucket"`
	MinioUseSSL       bool   `yaml:"minio_use_ssl"`
	DownloadBaseURL   string `yaml:"download_base_url"`
	DownloadURLExpiry int    `yaml:"download_url_expiry"`
	AuthVerifyURL     string `yaml:"auth_verify_url"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}
