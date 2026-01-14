package config

import "os"

func ExpandMap(data map[string]any, vars map[string]string) {
	for key, val := range data {
		data[key] = expandValue(val, vars)
	}
}

func expandValue(val any, vars map[string]string) any {
	switch v := val.(type) {
	case map[string]any:
		ExpandMap(v, vars)
		return v
	case []any:
		for i, item := range v {
			v[i] = expandValue(item, vars)
		}
		return v
	case string:
		return expandString(v, vars)
	default:
		return val
	}
}

func expandString(value string, vars map[string]string) string {
	return os.Expand(value, func(key string) string {
		if replacement, ok := vars[key]; ok {
			return replacement
		}
		return os.Getenv(key)
	})
}
