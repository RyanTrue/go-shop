package config

import (
	"flag"
	"github.com/caarlos0/env"
	"log"
)

type Config struct {
	RunAddress           string `env:"RUN_ADDRESS"`
	DatabaseURI          string `env:"DATABASE_URI"`
	AccrualSystemAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	JWTKey               string `env:"JWT_KEY"`
}

func NewConfig() (*Config, error) {
	c := &Config{}

	flag.StringVar(&c.RunAddress, "a", "", "адрес и порт запуска сервиса")
	flag.StringVar(&c.DatabaseURI, "d", "", "адрес системы расчёта начислений")
	flag.StringVar(&c.AccrualSystemAddress, "r", "", "адрес подключения к базе данных")

	flag.Parse()

	err := env.Parse(c)
	if err != nil {
		log.Fatal(err)
	}

	return c, nil
}
