package main

import (
	"fmt"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/kelseyhightower/envconfig"
	"gopkg.in/validator.v2"
	"gopkg.in/yaml.v3"
)

const yamlCfgName = "config.yml"

// Config struct contains all settings of the bot app
// yaml is local config, for development and testing.
// TODO: add default params where it is reasonable
type Config struct {
	Bot struct {
		Name  string `yaml:"Name" envconfig:"BOT_NAME"`
		Token string `yaml:"Token" envconfig:"BOT_TOKEN" validate:"nonzero"`
	} `yaml:"Bot"`

	Pg struct {
		Host   string `yaml:"Host" envconfig:"PG_HOST" validate:"nonzero"`
		Port   uint16 `yaml:"Port" envconfig:"PG_PORT" validate:"nonzero"`
		User   string `yaml:"User" envconfig:"PG_USER" validate:"nonzero"`
		Pass   string `yaml:"Pass" envconfig:"PG_PASS" validate:"nonzero"`
		DBName string `yaml:"DBName" envconfig:"PG_DB_NAME" validate:"nonzero"`
	} `yaml:"Postgres"`
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

func ParseConfig() (Config, error) {
	var cfg Config
	yamlErr := parseYamlConfig(&cfg)
	envErr := parseEnvConfig(&cfg)

	mErr := multierror.Append(yamlErr, envErr)

	if err := validator.Validate(cfg); err != nil {
		mErr = multierror.Append(mErr, fmt.Errorf("ParseConfig: Failed to extract all fields for config: %w", err))
		panic(mErr)
	}

	return cfg, mErr.ErrorOrNil()
}
