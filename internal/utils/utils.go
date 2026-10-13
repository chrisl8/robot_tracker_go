package utils

import "os"

// Abs returns the absolute value of x.
func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Max returns the larger of a and b.
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Min returns the smaller of a and b.
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// FileExists reports whether path can be stat-ed. Any error, not only "does not exist", counts as absent.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ToFloat64 converts a float64, float32, int or int64 to float64, and returns 0 for any other type.
func ToFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}
