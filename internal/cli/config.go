package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultAPIURL is used when TRUSTATTIC_API_URL is not set.
const DefaultAPIURL = "https://api.trustattic.com"

// ErrNotLoggedIn is returned by ResolveToken when no token is available from
// either TRUSTATTIC_TOKEN or the config file.
var ErrNotLoggedIn = errors.New("not logged in: run `trustattic login` or set TRUSTATTIC_TOKEN")

// Config is the on-disk shape of ~/.config/trustattic/config.yaml.
type Config struct {
	Token          string `yaml:"token,omitempty"`
	CurrentProject string `yaml:"current_project,omitempty"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "trustattic", "config.yaml"), nil
}

// LoadConfig reads the config file. A missing file is not an error; it
// returns the zero-value Config instead.
func LoadConfig() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// SaveConfig writes cfg to the config file, creating its parent directory as
// needed, with owner-only (0600) permissions.
func SaveConfig(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// ResolveToken returns the service-account token to authenticate with:
// TRUSTATTIC_TOKEN if set, else cfg.Token, else ErrNotLoggedIn.
func ResolveToken(cfg Config) (string, error) {
	if v := os.Getenv("TRUSTATTIC_TOKEN"); v != "" {
		return v, nil
	}
	if cfg.Token != "" {
		return cfg.Token, nil
	}
	return "", ErrNotLoggedIn
}

// ResolveAPIURL returns TRUSTATTIC_API_URL if set, else DefaultAPIURL.
func ResolveAPIURL() string {
	if v := os.Getenv("TRUSTATTIC_API_URL"); v != "" {
		return v
	}
	return DefaultAPIURL
}
