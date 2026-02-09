package utils

import "os"

func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func AbsFloat64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func MaxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func MinFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

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

func ContainsPath(path, target string) bool {
	if len(path) < len(target) {
		return false
	}
	for i := 0; i <= len(path)-len(target); i++ {
		if path[i:i+len(target)] == target {
			return true
		}
	}
	return false
}
