package config

import "strings"

func LookupLiquibaseExec(path string) (string, bool, error) {
	data, err := readConfigMap(path)
	if err != nil {
		return "", false, err
	}
	liquibase, ok := data["liquibase"].(map[string]any)
	if !ok {
		return "", false, nil
	}
	raw, ok := liquibase["exec"]
	if !ok {
		return "", false, nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", false, nil
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false, nil
	}
	return value, true, nil
}

func LookupLiquibaseExecMode(path string) (string, bool, error) {
	data, err := readConfigMap(path)
	if err != nil {
		return "", false, err
	}
	liquibase, ok := data["liquibase"].(map[string]any)
	if !ok {
		return "", false, nil
	}
	raw, ok := liquibase["exec_mode"]
	if !ok {
		return "", false, nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", false, nil
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false, nil
	}
	return value, true, nil
}
