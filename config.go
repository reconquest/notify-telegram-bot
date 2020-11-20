package main

import (
	"github.com/kovetskiy/ko"
)

type Config struct {
	TelegramBotToken string `toml:"telegrambot_token"`
	DatabaseURI      string `toml:"uri_db" env:"DATABASE_URI"`
	DatabaseName     string `toml:"database_name"`
}

func LoadConfig(path string) (*Config, error) {
	config := &Config{}
	err := ko.Load(path, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}
