package config

import "strings"

func LookupDBMSImage(path string) (string, bool, error) {
	data, err := readConfigMap(path)
	if err != nil {
		return "", false, err
	}
	dbms, ok := data["dbms"].(map[string]any)
	if !ok {
		return "", false, nil
	}
	raw, ok := dbms["image"]
	if !ok {
		return "", false, nil
	}
	image, ok := raw.(string)
	if !ok {
		return "", false, nil
	}
	image = strings.TrimSpace(image)
	if image == "" {
		return "", false, nil
	}
	return image, true, nil
}
