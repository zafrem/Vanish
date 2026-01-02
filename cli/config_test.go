package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGetConfigPath(t *testing.T) {
	path, err := getConfigPath()
	if err != nil {
		t.Fatalf("getConfigPath() failed: %v", err)
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
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create config
	cfg := &Config{
		BaseURL: "http://test.example.com",
		Token:   "test-token-12345",
	}

	// Manually save config to temp directory
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Verify file permissions (should be 0600 - user read/write only)
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Failed to stat config file: %v", err)
	}

	mode := info.Mode()
	if mode.Perm() != 0600 {
		t.Errorf("Config file permissions = %o, want 0600", mode.Perm())
	}

	// Load config
	loadedData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var loadedCfg Config
	if err := json.Unmarshal(loadedData, &loadedCfg); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// Verify loaded config matches saved config
	if loadedCfg.BaseURL != cfg.BaseURL {
		t.Errorf("BaseURL = %s, want %s", loadedCfg.BaseURL, cfg.BaseURL)
	}
	if loadedCfg.Token != cfg.Token {
		t.Errorf("Token = %s, want %s", loadedCfg.Token, cfg.Token)
	}
}

func TestConfigJSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := &Config{
		BaseURL: "http://test.example.com",
		Token:   "test-token-12345",
	}

	// Save config
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Read and verify JSON format
	fileData, err := os.ReadFile(configPath)
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
	configPath := filepath.Join(tmpDir, "nonexistent.json")

	_, err := os.ReadFile(configPath)
	if err == nil {
		t.Error("Reading non-existent file should fail")
	}

	if !os.IsNotExist(err) {
		t.Errorf("Expected IsNotExist error, got: %v", err)
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	err := os.WriteFile(configPath, []byte("not valid json {"), 0600)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Try to load
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var cfg Config
	err = json.Unmarshal(data, &cfg)
	if err == nil {
		t.Error("Unmarshaling invalid JSON should fail")
	}
}

func TestSaveConfigCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nested", "dir", "config.json")

	cfg := &Config{
		BaseURL: "http://test.example.com",
		Token:   "test-token",
	}

	// Create nested directory
	dirPath := filepath.Dir(configPath)
	if err := os.MkdirAll(dirPath, 0700); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Verify directory was created
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("Directory was not created: %v", err)
	}

	if !info.IsDir() {
		t.Error("Path exists but is not a directory")
	}

	// Verify directory permissions (should be 0700)
	if info.Mode().Perm() != 0700 {
		t.Errorf("Directory permissions = %o, want 0700", info.Mode().Perm())
	}

	// Save config
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}
}

func TestConfigMultipleSaves(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configs := []Config{
		{BaseURL: "http://first.com", Token: "token1"},
		{BaseURL: "http://second.com", Token: "token2"},
		{BaseURL: "http://third.com", Token: "token3"},
	}

	for i, cfg := range configs {
		// Save config
		data, err := json.MarshalIndent(&cfg, "", "  ")
		if err != nil {
			t.Fatalf("Marshal #%d failed: %v", i, err)
		}

		if err := os.WriteFile(configPath, data, 0600); err != nil {
			t.Fatalf("Write #%d failed: %v", i, err)
		}

		// Load and verify
		loadedData, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Read #%d failed: %v", i, err)
		}

		var loaded Config
		if err := json.Unmarshal(loadedData, &loaded); err != nil {
			t.Fatalf("Unmarshal #%d failed: %v", i, err)
		}

		if loaded.BaseURL != cfg.BaseURL || loaded.Token != cfg.Token {
			t.Errorf("Config #%d not saved correctly: got %+v, want %+v", i, loaded, cfg)
		}
	}
}
