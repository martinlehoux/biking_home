package main

import (
	"os"
	"strings"

	"github.com/martinlehoux/kagamigo/kcore"
)

func loadEnv() map[string]string {
	values := map[string]string{}
	data, err := os.ReadFile(".env")
	if err != nil {
		return values
	}
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
	return values
}

func updateEnv(updates map[string]string) {
	data, err := os.ReadFile(".env")
	kcore.Expect(err, "failed to read .env")
	lines := strings.Split(string(data), "\n")
	for key, value := range updates {
		key = strings.TrimSpace(key)
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
	err = os.WriteFile(".env", []byte(strings.Join(lines, "\n")+"\n"), 0o600)
	kcore.Expect(err, "failed to write .env")
}
