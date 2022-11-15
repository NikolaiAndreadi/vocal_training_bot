package main

import (
	"fmt"
	"os"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"gopkg.in/validator.v2"
	"gopkg.in/yaml.v3"
)

const yamlCfgName = "config.yml"

// Config struct contains all settings of the bot app
// yaml is local config, for development and testing.
// TODO: add default params where it is reasonable
type Config struct {
	Bot struct {
		Token         string `yaml:"Token" envconfig:"BOT_TOKEN" validate:"nonzero"`
		ProviderToken string `yaml:"ProviderToken" envconfig:"PROVIDER_TOKEN" validate:"nonzero"`
	} `yaml:"Bot"`

	Pg struct {
		Host   string `yaml:"Host" envconfig:"PG_HOST" validate:"nonzero" default:"localhost"`
		Port   string `yaml:"Port" envconfig:"PG_PORT" validate:"nonzero"`
		User   string `yaml:"User" envconfig:"PG_USER" validate:"nonzero"`
		Pass   string `yaml:"Pass" envconfig:"PG_PASS"`
		DBName string `yaml:"DBName" envconfig:"PG_DB_NAME" validate:"nonzero"`
	} `yaml:"Postgres"`

	Redis struct {
		Host string `yaml:"Host" envconfig:"REDIS_HOST" validate:"nonzero" default:"localhost"`
		Port string `yaml:"Port" envconfig:"REDIS_PORT" validate:"nonzero"`
		Pass string `yaml:"Pass" envconfig:"REDIS_PASS"`
	} `yaml:"Redis"`
}

func parseYamlConfig(cfg *Config) error {
	f, err := os.Open(yamlCfgName)
	if err != nil {
		return fmt.Errorf("parseYamlConfig: Can't open config.yml: %w", err)
	}
	defer func(f *os.File) {
		if err := f.Close(); err != nil {
			err = fmt.Errorf("parseYamlConfig: Error while closing file: %w", err)
		}
	}(f)

	decoder := yaml.NewDecoder(f)
	if err = decoder.Decode(cfg); err != nil {
		return fmt.Errorf("parseYamlConfig: Can't decode config.yml: %w", err)
	}
	return nil
}

func parseEnvConfig(cfg *Config) error {
	err := envconfig.Process("", cfg)
	if err != nil {
		return fmt.Errorf("parseEnvConfig: %w", err)
	}
	return nil
}

func ParseConfig() Config {
	var cfg Config
	yamlErr := parseYamlConfig(&cfg)
	envErr := parseEnvConfig(&cfg)

	if envErr != nil {
		if yamlErr != nil {
			logger.Error("yaml parse error", zap.Error(yamlErr))
		}
		logger.Error("env parse error", zap.Error(envErr))
	}
	if err := validator.Validate(cfg); err != nil {
		err = fmt.Errorf("ParseConfig: Failed to extract all fields for config: %w", err)
		logger.Fatal("", zap.Error(err))
		panic(err)
	}

	return cfg
}
