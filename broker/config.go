package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Broker BrokerConfig `yaml:"broker"`
	DB     DBConfig     `yaml:"db"`
}

type BrokerConfig struct {
	Addr string `yaml:"addr"`
}

type DBConfig struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Name     string `yaml:"name"`
	URL      string `yaml:"url"` // if set, overrides all other fields
}

func (d DBConfig) ConnString() string {
	if d.URL != "" {
		return d.URL
	}
	if d.Password != "" {
		return fmt.Sprintf("postgres://%s:%s@%s:%s/%s", d.User, d.Password, d.Host, d.Port, d.Name)
	}
	return fmt.Sprintf("postgres://%s@%s:%s/%s", d.User, d.Host, d.Port, d.Name)
}

func loadConfig(path string) (Config, error) {
	cfg := Config{
		Broker: BrokerConfig{Addr: "localhost:7070"},
		DB:     DBConfig{User: "postgres", Host: "localhost", Port: "5432", Name: "nito"},
	}
	f, err := os.Open(path)
	if err != nil {
		return cfg, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}
