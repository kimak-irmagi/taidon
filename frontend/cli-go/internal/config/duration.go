package config

import "time"

func ParseDuration(raw string, fallback time.Duration) (time.Duration, error) {
	if raw == "" {
		return fallback, nil
	}
	return time.ParseDuration(raw)
}
