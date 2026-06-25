package main

import (
	"log/slog"
	"os"
	"strconv"
)

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envOrDefaultInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		slog.Warn("Invalid env var value, using default", "key", key, "value", v, "default", def)
		return def
	}
	return n
}
