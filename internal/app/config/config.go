package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Error loading .env file: %v\n", err)
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "local"
	}

	err = loadConfig(env)
	if err != nil {
		log.Fatalf("Failed to load configuration for %s environment: %s", env, err.Error())
	}

}

func loadConfig(env string) error {
	viper.SetConfigName(fmt.Sprintf("config.%s", env))
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("error reading config file: %s", err.Error())
	}

	return nil
}

func NewInternalConfig() *InternalConfig {
	var config InternalConfig
	err := viper.UnmarshalKey("internal_config", &config)
	if err != nil {
		log.Fatalf("unable to decode into internalConfig struct: %s", err.Error())
	}
	return &config
}

func NewDriverConfig() *DriverConfig {
	var config DriverConfig
	err := viper.UnmarshalKey("driver_config", &config)
	if err != nil {
		log.Fatalf("unable to decode into driverConfig struct: %s", err.Error())
	}
	return &config
}
