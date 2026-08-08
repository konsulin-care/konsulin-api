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

// envNumeric parses an env var as a signed or unsigned integer of the given
// bit size, falling back to defaultValue when the key is missing, empty, or
// unparsable. parse is the strconv parse function for the target type.
func envNumeric[T ~int64 | ~uint64](key string, defaultValue T, bitSize int, parse func(string, int, int) (T, error)) T {
	value, ok := lookupEnvWithDefault(key)
	if !ok {
		return defaultValue
	}
	parsed, err := parse(value, 10, bitSize)
	if err != nil {
		log.Printf(constvars.ErrEnvParsing, key, err)
		return defaultValue
	}
	return parsed
}

func envInt64(key string, defaultValue int64, bitSize int) int64 {
	return envNumeric(key, defaultValue, bitSize, func(s string, base, bits int) (int64, error) {
		return strconv.ParseInt(s, base, bits)
	})
}

func envUint64(key string, defaultValue uint64, bitSize int) uint64 {
	return envNumeric(key, defaultValue, bitSize, func(s string, base, bits int) (uint64, error) {
		return strconv.ParseUint(s, base, bits)
	})
}

func GetEnvString(key, defaultValue string) string {
	value, ok := lookupEnvWithDefault(key)
	if !ok {
		return defaultValue
	}
	return value
}

func GetEnvInt(key string, defaultValue int) int {
	return int(envInt64(key, int64(defaultValue), 0))
}

func GetEnvInt64(key string, defaultValue int64) int64 {
	return envInt64(key, defaultValue, 64)
}

func GetEnvInt32(key string, defaultValue int32) int32 {
	return int32(envInt64(key, int64(defaultValue), 32))
}

func GetEnvUint(key string, defaultValue uint) uint {
	return uint(envUint64(key, uint64(defaultValue), 0))
}

func GetEnvUint64(key string, defaultValue uint64) uint64 {
	return envUint64(key, defaultValue, 64)
}

func GetEnvUint32(key string, defaultValue uint32) uint32 {
	return uint32(envUint64(key, uint64(defaultValue), 32))
}

func GetEnvUint16(key string, defaultValue uint16) uint16 {
	return uint16(envUint64(key, uint64(defaultValue), 16))
}

func GetEnvUint8(key string, defaultValue uint8) uint8 {
	return uint8(envUint64(key, uint64(defaultValue), 8))
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
