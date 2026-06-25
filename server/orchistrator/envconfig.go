package main

import (
	"log"
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
		log.Printf("Warning: invalid value for %s=%q, using default %d", key, v, def)
		return def
	}
	return n
}
