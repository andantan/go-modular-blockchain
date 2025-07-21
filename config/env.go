package config

import (
	"github.com/joho/godotenv"
	"log"
	"os"
	"strconv"
)

func InitEnv() {
	err := godotenv.Load()

	if err != nil {
		log.Printf("Could not load .env file. Proceeding without it (assuming environment variables are set externally): %v", err)
		log.Fatalf("Failed to load .env file: %v", err)
	} else {
		log.Println("Environment variables successfully loaded from .env file.")
	}
}

func GetEnvVar(key string) string {
	value := os.Getenv(key)

	if value == "" {
		log.Fatalf("Required environment variable \"%s\" is not defined. Application will exit.", key)
	}

	return value
}

func GetIntEnvVar(key string) int {
	stringValue := GetEnvVar(key)
	intValue, err := strconv.Atoi(stringValue)

	if err != nil {
		log.Fatalf("Environment variable \"%s\" (\"%s\") is not a valid integer. Application will exit. Error: %v", key, stringValue, err)
	}

	return intValue
}
