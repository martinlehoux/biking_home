package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Database DatabaseConfig `yaml:"database"`
	Server   ServerConfig   `yaml:"server"`
	Storage  StorageConfig  `yaml:"storage"`
	Strava   StravaConfig   `yaml:"strava"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type ServerConfig struct {
	Address   string `yaml:"address"`
	PublicURL string `yaml:"public_url"`
}

type StorageConfig struct {
	GPXDir string `yaml:"gpx_dir"`
}

type StravaConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	AccessToken  string `yaml:"access_token"`
	RefreshToken string `yaml:"refresh_token"`
	ExpiresAt    int64  `yaml:"expires_at"`
}

func Default() Config {
	return Config{
		Database: DatabaseConfig{Path: "biking_home.db"},
		Server: ServerConfig{
			Address:   "127.0.0.1:8080",
			PublicURL: "http://localhost:8080",
		},
		Storage: StorageConfig{GPXDir: "data/rides"},
	}
}

func Load(filename string) (Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", filename, err)
	}
	config := Default()
	if err := yaml.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", filename, err)
	}
	if config.Database.Path == "" {
		return Config{}, fmt.Errorf("config %q: database.path is required", filename)
	}
	if config.Server.Address == "" || config.Server.PublicURL == "" {
		return Config{}, fmt.Errorf("config %q: server.address and server.public_url are required", filename)
	}
	if config.Storage.GPXDir == "" {
		return Config{}, fmt.Errorf("config %q: storage.gpx_dir is required", filename)
	}
	return config, nil
}

func Save(filename string, config Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	directory := filepath.Dir(filename)
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(filename)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	temporaryName := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryName)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set config permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary config: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Rename(temporaryName, filename); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	removeTemporary = false
	return syncDirectory(directory)
}

func syncDirectory(directory string) error {
	dir, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("open config directory: %w", err)
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("sync config directory: %w", err)
	}
	return nil
}
