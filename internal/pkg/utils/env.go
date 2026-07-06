package utils

import (
	"konsulin-service/internal/pkg/constvars"
	"log"
	"os"
	"strconv"
)

// lookupEnvWithDefault reads an env var and returns whether the key existed.
// Returns the raw value (or empty if missing) and true if the key was present.
func lookupEnvWithDefault(key string) (string, bool) {
	value, exists := os.LookupEnv(key)
	if !exists {
		log.Printf(constvars.ErrEnvKeyNotExist, key)
		return "", false
	}
	if value == "" {
		return "", false
	}
	return value, true
}

func GetEnvString(key, defaultValue string) string {
	value, ok := lookupEnvWithDefault(key)
	if !ok {
		return defaultValue
	}
	return value
}

func GetEnvInt(key string, defaultValue int) int {
	value, ok := lookupEnvWithDefault(key)
	if !ok {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		log.Printf(constvars.ErrEnvParsing, key, err)
		return defaultValue
	}
	return intValue
}

func GetEnvInt64(key string, defaultValue int64) int64 {
	value, ok := lookupEnvWithDefault(key)
	if !ok {
		return defaultValue
	}
	int64Value, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		log.Printf(constvars.ErrEnvParsing, key, err)
		return defaultValue
	}
	return int64Value
}

func GetEnvInt32(key string, defaultValue int32) int32 {
	value, ok := lookupEnvWithDefault(key)
	if !ok {
		return defaultValue
	}
	int32Value, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		log.Printf(constvars.ErrEnvParsing, key, err)
		return defaultValue
	}
	return int32(int32Value)
}

func GetEnvUint(key string, defaultValue uint) uint {
	value, ok := lookupEnvWithDefault(key)
	if !ok {
		return defaultValue
	}
	uintValue, err := strconv.ParseUint(value, 10, 0)
	if err != nil {
		log.Printf(constvars.ErrEnvParsing, key, err)
		return defaultValue
	}
	return uint(uintValue)
}

func GetEnvUint64(key string, defaultValue uint64) uint64 {
	value, ok := lookupEnvWithDefault(key)
	if !ok {
		return defaultValue
	}
	uint64Value, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		log.Printf(constvars.ErrEnvParsing, key, err)
		return defaultValue
	}
	return uint64Value
}

func GetEnvUint32(key string, defaultValue uint32) uint32 {
	value, ok := lookupEnvWithDefault(key)
	if !ok {
		return defaultValue
	}
	uint32Value, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		log.Printf(constvars.ErrEnvParsing, key, err)
		return defaultValue
	}
	return uint32(uint32Value)
}

func GetEnvUint16(key string, defaultValue uint16) uint16 {
	value, ok := lookupEnvWithDefault(key)
	if !ok {
		return defaultValue
	}
	uint16Value, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		log.Printf(constvars.ErrEnvParsing, key, err)
		return defaultValue
	}
	return uint16(uint16Value)
}

func GetEnvUint8(key string, defaultValue uint8) uint8 {
	value, ok := lookupEnvWithDefault(key)
	if !ok {
		return defaultValue
	}
	uint8Value, err := strconv.ParseUint(value, 10, 8)
	if err != nil {
		log.Printf(constvars.ErrEnvParsing, key, err)
		return defaultValue
	}
	return uint8(uint8Value)
}

func GetEnvBool(key string, defaultValue bool) bool {
	value, ok := lookupEnvWithDefault(key)
	if !ok {
		return defaultValue
	}
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		log.Printf(constvars.ErrEnvParsing, key, err)
		return defaultValue
	}
	return boolValue
}

func GetEnvFloat(key string, defaultValue float64) float64 {
	value, ok := lookupEnvWithDefault(key)
	if !ok {
		return defaultValue
	}
	floatValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		log.Printf(constvars.ErrEnvParsing, key, err)
		return defaultValue
	}
	return floatValue
}
