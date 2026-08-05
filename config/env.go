package config

import (
	"fmt"
	"os"
	"strings"
)

func LoadEnv(filename string) (map[string]string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if found {
			values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"`)
		}
	}
	return values, nil
}

func UpdateEnv(filename string, updates map[string]string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	for key, value := range updates {
		found := false
		for i, line := range lines {
			existingKey, _, cut := strings.Cut(strings.TrimSpace(line), "=")
			if cut && strings.TrimSpace(existingKey) == key {
				lines[i] = key + "=" + value
				found = true
				break
			}
		}
		if !found {
			lines = append(lines, key+"="+value)
		}
	}
	if err := os.WriteFile(filename, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		return fmt.Errorf("write env file: %w", err)
	}
	return nil
}
