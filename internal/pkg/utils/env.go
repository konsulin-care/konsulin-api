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
	case int64:
		int64Value, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			log.Printf("Error parsing %s: %v, will use default value", key, err)
			return defaultValue
		}
		return int64Value
	case int32:
		int32Value, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			log.Printf("Error parsing %s: %v, will use default value", key, err)
			return defaultValue
		}
		return int32(int32Value)
	case uint:
		uintValue, err := strconv.ParseUint(value, 10, 0)
		if err != nil {
			log.Printf("Error parsing %s: %v, will use default value", key, err)
			return defaultValue
		}
		return uint(uintValue)
	case uint64:
		uint64Value, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			log.Printf("Error parsing %s: %v, will use default value", key, err)
			return defaultValue
		}
		return uint64Value
	case uint32:
		uint32Value, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			log.Printf("Error parsing %s: %v, will use default value", key, err)
			return defaultValue
		}
		return uint32(uint32Value)
	case uint16:
		uint16Value, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			log.Printf("Error parsing %s: %v, will use default value", key, err)
			return defaultValue
		}
		return uint16(uint16Value)
	case uint8:
		uint8Value, err := strconv.ParseUint(value, 10, 8)
		if err != nil {
			log.Printf("Error parsing %s: %v, will use default value", key, err)
			return defaultValue
		}
		return uint8(uint8Value)
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

func GetEnvInt64(key string, defaultValue int64) int64 {
	return getEnv(key, defaultValue).(int64)
}

func GetEnvInt32(key string, defaultValue int32) int32 {
	return getEnv(key, defaultValue).(int32)
}

func GetEnvUint(key string, defaultValue uint) uint {
	return getEnv(key, defaultValue).(uint)
}

func GetEnvUint64(key string, defaultValue uint64) uint64 {
	return getEnv(key, defaultValue).(uint64)
}

func GetEnvUint32(key string, defaultValue uint32) uint32 {
	return getEnv(key, defaultValue).(uint32)
}

func GetEnvUint16(key string, defaultValue uint16) uint16 {
	return getEnv(key, defaultValue).(uint16)
}

func GetEnvUint8(key string, defaultValue uint8) uint8 {
	return getEnv(key, defaultValue).(uint8)
}

func GetEnvBool(key string, defaultValue bool) bool {
	return getEnv(key, defaultValue).(bool)
}

func GetEnvFloat(key string, defaultValue float64) float64 {
	return getEnv(key, defaultValue).(float64)
}
