package utils

import "testing"

func TestGetEnvString(t *testing.T) {
	t.Run("key exists", func(t *testing.T) {
		t.Setenv("TEST_STRING", "hello")
		if got := GetEnvString("TEST_STRING", "default"); got != "hello" {
			t.Errorf("GetEnvString() = %q, want %q", got, "hello")
		}
	})

	t.Run("key missing returns default", func(t *testing.T) {
		t.Setenv("TEST_STRING_MISSING", "")
		if got := GetEnvString("TEST_STRING_MISSING", "default"); got != "default" {
			t.Errorf("GetEnvString() = %q, want %q", got, "default")
		}
	})

	t.Run("empty value returns default", func(t *testing.T) {
		t.Setenv("TEST_STRING_EMPTY", "")
		if got := GetEnvString("TEST_STRING_EMPTY", "default"); got != "default" {
			t.Errorf("GetEnvString() = %q, want %q", got, "default")
		}
	})
}

func TestGetEnvInt(t *testing.T) {
	t.Run("key exists valid int", func(t *testing.T) {
		t.Setenv("TEST_INT", "42")
		if got := GetEnvInt("TEST_INT", 0); got != 42 {
			t.Errorf("GetEnvInt() = %d, want %d", got, 42)
		}
	})

	t.Run("key missing returns default", func(t *testing.T) {
		t.Setenv("TEST_INT_MISSING", "")
		if got := GetEnvInt("TEST_INT_MISSING", 99); got != 99 {
			t.Errorf("GetEnvInt() = %d, want %d", got, 99)
		}
	})

	t.Run("invalid int returns default", func(t *testing.T) {
		t.Setenv("TEST_INT_INVALID", "notanumber")
		if got := GetEnvInt("TEST_INT_INVALID", 7); got != 7 {
			t.Errorf("GetEnvInt() = %d, want %d", got, 7)
		}
	})
}

func TestGetEnvBool(t *testing.T) {
	t.Run("key exists true", func(t *testing.T) {
		t.Setenv("TEST_BOOL", "true")
		if got := GetEnvBool("TEST_BOOL", false); got != true {
			t.Errorf("GetEnvBool() = %v, want %v", got, true)
		}
	})

	t.Run("key exists false", func(t *testing.T) {
		t.Setenv("TEST_BOOL", "false")
		if got := GetEnvBool("TEST_BOOL", true); got != false {
			t.Errorf("GetEnvBool() = %v, want %v", got, false)
		}
	})

	t.Run("invalid bool returns default", func(t *testing.T) {
		t.Setenv("TEST_BOOL_INVALID", "maybe")
		if got := GetEnvBool("TEST_BOOL_INVALID", true); got != true {
			t.Errorf("GetEnvBool() = %v, want %v", got, true)
		}
	})
}

func TestGetEnvFloat(t *testing.T) {
	t.Run("key exists valid float", func(t *testing.T) {
		t.Setenv("TEST_FLOAT", "3.14")
		if got := GetEnvFloat("TEST_FLOAT", 0); got != 3.14 {
			t.Errorf("GetEnvFloat() = %f, want %f", got, 3.14)
		}
	})

	t.Run("invalid float returns default", func(t *testing.T) {
		t.Setenv("TEST_FLOAT_INVALID", "notafloat")
		if got := GetEnvFloat("TEST_FLOAT_INVALID", 1.5); got != 1.5 {
			t.Errorf("GetEnvFloat() = %f, want %f", got, 1.5)
		}
	})
}

func TestGetEnvInt64(t *testing.T) {
	t.Run("key exists valid int64", func(t *testing.T) {
		t.Setenv("TEST_INT64", "9223372036854775807")
		if got := GetEnvInt64("TEST_INT64", 0); got != 9223372036854775807 {
			t.Errorf("GetEnvInt64() = %d, want %d", got, 9223372036854775807)
		}
	})
}

func TestGetEnvUint(t *testing.T) {
	t.Run("key exists valid uint", func(t *testing.T) {
		t.Setenv("TEST_UINT", "100")
		if got := GetEnvUint("TEST_UINT", 0); got != 100 {
			t.Errorf("GetEnvUint() = %d, want %d", got, 100)
		}
	})
}

func TestGetEnvUint64(t *testing.T) {
	t.Run("key exists valid uint64", func(t *testing.T) {
		t.Setenv("TEST_UINT64", "18446744073709551615")
		if got := GetEnvUint64("TEST_UINT64", 0); got != 18446744073709551615 {
			t.Errorf("GetEnvUint64() = %d, want %d", got, uint64(18446744073709551615))
		}
	})
}
