package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultAPIURL is used when TRUSTATTIC_API_URL is not set. It must include
// the spec's `/api/v2` base path: the vendored spec declares it under
// `servers:`, and the platform mounts every route beneath it, but
// oapi-codegen's generated client does not embed the `servers:` block - the
// caller-supplied base URL has to carry the full prefix itself.
const DefaultAPIURL = "https://api.trustattic.com/api/v2"

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
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	// MkdirAll's and WriteFile's mode arguments only apply on *creation*
	// (and are masked by umask even then). This file holds a service-account
	// token, so tighten both explicitly on every save - otherwise a config
	// file or directory that already exists with looser permissions keeps
	// them forever, however it came to be that way.
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("secure config dir: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("secure config file: %w", err)
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
