package hassdev

import (
	"os"
	"strconv"
)

func getenv(key string) string {
	return os.Getenv(key)
}

func envFloatOr(key string, fallback float64) float64 {
	v := getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}
