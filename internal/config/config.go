package config

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	Env            string     `yaml:"env" env-default:"local"`
	GRPC           GRPCConfig `yaml:"grpc"`
	MigrationsPath string
	TokenTTL       time.Duration `yaml:"token_ttl" env-default:"1h"`
}

type GRPCConfig struct {
	Port    int           `yaml:"port"`
	Timeout time.Duration `yaml:"timeout"`
}

func MustLoad() *Config {
	configPath := fetchConfig()
	if configPath == "" {
		panic("config file path is empty")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic("config file does not exist")
	}

	var config Config

	if err := cleanenv.ReadConfig(configPath, &config); err != nil {
		panic("config file read error: " + err.Error())
	}

	return &config
}

func fetchConfig() string {
	var result string

	if err := godotenv.Load(".env"); err != nil {
		fmt.Println("⚠️  .env not loaded:", err)
	} else {
		fmt.Println("✅ .env loaded successfully")
	}

	fmt.Println("CONFIG_PATH from env:", os.Getenv("CONFIG_PATH"))

	flag.StringVar(&result, "config", "", "config file path")
	flag.Parse()

	if result == "" {
		result = os.Getenv("CONFIG_PATH")
	}

	if result == "" {
		result = "./config/local.yaml"
	}

	return result
}
