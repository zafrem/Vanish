package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGetConfigPath(t *testing.T) {
	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath() failed: %v", err)
	}

	if path == "" {
		t.Error("Config path is empty")
	}

	// Should be absolute path
	if !filepath.IsAbs(path) {
		t.Error("Config path should be absolute")
	}

	t.Logf("Config path: %s", path)
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Create temporary test directory
	tmpDir := t.TempDir()

	// Override config path for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg := &Config{
		BaseURL: "http://test.example.com",
		Token:   "test-token-12345",
	}

	// Save config
	err := SaveConfig(cfg)
	if err != nil {
		t.Fatalf("SaveConfig() failed: %v", err)
	}

	// Load config
	loadedCfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Verify loaded config matches saved config
	if loadedCfg.BaseURL != cfg.BaseURL {
		t.Errorf("BaseURL = %s, want %s", loadedCfg.BaseURL, cfg.BaseURL)
	}
	if loadedCfg.Token != cfg.Token {
		t.Errorf("Token = %s, want %s", loadedCfg.Token, cfg.Token)
	}
}

func TestSaveConfigFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg := &Config{
		BaseURL: "http://test.example.com",
		Token:   "test-token-12345",
	}

	// Save config
	err := SaveConfig(cfg)
	if err != nil {
		t.Fatalf("SaveConfig() failed: %v", err)
	}

	// Get config path
	path, _ := GetConfigPath()

	// Verify file permissions (should be 0600 - user read/write only)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to stat config file: %v", err)
	}

	mode := info.Mode()
	if mode.Perm() != 0600 {
		t.Errorf("Config file permissions = %o, want 0600", mode.Perm())
	}

	// Verify directory permissions (should be 0700)
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Failed to stat config directory: %v", err)
	}

	if dirInfo.Mode().Perm() != 0700 {
		t.Errorf("Directory permissions = %o, want 0700", dirInfo.Mode().Perm())
	}
}

func TestConfigJSONFormat(t *testing.T) {
	tmpDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg := &Config{
		BaseURL: "http://test.example.com",
		Token:   "test-token-12345",
	}

	// Save config
	err := SaveConfig(cfg)
	if err != nil {
		t.Fatalf("SaveConfig() failed: %v", err)
	}

	// Get config path and read file
	path, _ := GetConfigPath()
	fileData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal(fileData, &parsed)
	if err != nil {
		t.Fatalf("Config file is not valid JSON: %v", err)
	}

	// Verify expected fields
	if parsed["base_url"] != cfg.BaseURL {
		t.Errorf("base_url = %v, want %s", parsed["base_url"], cfg.BaseURL)
	}
	if parsed["token"] != cfg.Token {
		t.Errorf("token = %v, want %s", parsed["token"], cfg.Token)
	}
}

func TestLoadConfigNotExists(t *testing.T) {
	tmpDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Try to load non-existent config
	_, err := LoadConfig()
	if err == nil {
		t.Error("LoadConfig() should fail when config doesn't exist")
	}

	// Should contain helpful error message
	if err != nil && !os.IsNotExist(err) {
		// Check that error mentions the config command
		errMsg := err.Error()
		if errMsg == "" {
			t.Error("Error message should not be empty")
		}
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Get config path and create directory
	path, _ := GetConfigPath()
	os.MkdirAll(filepath.Dir(path), 0700)

	// Write invalid JSON
	err := os.WriteFile(path, []byte("not valid json {"), 0600)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Try to load
	_, err = LoadConfig()
	if err == nil {
		t.Error("LoadConfig() should fail with invalid JSON")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  &Config{BaseURL: "http://test.com", Token: "token123"},
			wantErr: false,
		},
		{
			name:    "missing baseURL",
			config:  &Config{BaseURL: "", Token: "token123"},
			wantErr: true,
		},
		{
			name:    "missing token",
			config:  &Config{BaseURL: "http://test.com", Token: ""},
			wantErr: true,
		},
		{
			name:    "trailing slash removed",
			config:  &Config{BaseURL: "http://test.com/", Token: "token123"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Verify trailing slash is removed
			if tt.name == "trailing slash removed" && err == nil {
				if tt.config.BaseURL != "http://test.com" {
					t.Errorf("BaseURL = %s, want http://test.com", tt.config.BaseURL)
				}
			}
		})
	}
}

func TestSaveConfigNil(t *testing.T) {
	err := SaveConfig(nil)
	if err == nil {
		t.Error("SaveConfig(nil) should return error")
	}
}

func TestMultipleSaves(t *testing.T) {
	tmpDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	configs := []Config{
		{BaseURL: "http://first.com", Token: "token1"},
		{BaseURL: "http://second.com", Token: "token2"},
		{BaseURL: "http://third.com", Token: "token3"},
	}

	for i, cfg := range configs {
		// Save config
		err := SaveConfig(&cfg)
		if err != nil {
			t.Fatalf("SaveConfig #%d failed: %v", i, err)
		}

		// Load and verify
		loaded, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig #%d failed: %v", i, err)
		}

		if loaded.BaseURL != cfg.BaseURL || loaded.Token != cfg.Token {
			t.Errorf("Config #%d not saved correctly: got %+v, want %+v", i, loaded, cfg)
		}
	}
}
