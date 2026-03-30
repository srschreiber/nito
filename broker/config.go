// Copyright 2026 Sam Schreiber
//
// This file is part of nito.
//
// nito is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// nito is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with nito. If not, see <https://www.gnu.org/licenses/>.

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
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.Broker.Addr == "" {
		cfg.Broker.Addr = "localhost:7070"
	}
	if cfg.DB.Port == "" {
		cfg.DB.Port = "5432"
	}
	return cfg, nil
}
