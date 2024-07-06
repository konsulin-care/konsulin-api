package utils

import (
	"log"
	"os"
	"strconv"
)

func getEnv(key string, defaultValue interface{}) interface{} {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}

	switch defaultValue.(type) {
	case string:
		return value
	case int:
		intValue, err := strconv.Atoi(value)
		if err != nil {
			log.Printf("Error parsing %s: %v, will use default value", key, err)
			return defaultValue
		}
		return intValue
	case bool:
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			log.Printf("Error parsing %s: %v, will use default value", key, err)
			return defaultValue
		}
		return boolValue
	case float64:
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			log.Printf("Error parsing %s: %v, will use default value", key, err)
			return defaultValue
		}
		return floatValue
	default:
		return defaultValue
	}
}

func GetEnvString(key, defaultValue string) string {
	return getEnv(key, defaultValue).(string)
}

func GetEnvInt(key string, defaultValue int) int {
	return getEnv(key, defaultValue).(int)
}

func GetEnvBool(key string, defaultValue bool) bool {
	return getEnv(key, defaultValue).(bool)
}

func GetEnvFloat(key string, defaultValue float64) float64 {
	return getEnv(key, defaultValue).(float64)
}
