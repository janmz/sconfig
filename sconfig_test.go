package sconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestConfig represents a test configuration structure
type TestConfig struct {
	Version                int    `json:"version" default:"1"`
	DatabaseHost           string `json:"database_host" default:"localhost"`
	DatabasePort           int    `json:"database_port" default:"5432"`
	DatabaseName           string `json:"database_name" default:"testdb"`
	DatabaseUser           string `json:"database_user" default:"testuser"`
	DatabasePassword       string `json:"database_password"`
	DatabaseSecurePassword string `json:"database_secure_password"`
	APIKey                 string `json:"api_key"`
	APISecureKey           string `json:"api_secure_key"`
	Debug                  bool   `json:"debug" default:"true"`
}

// NestedTestConfig represents a nested configuration structure
type NestedTestConfig struct {
	Version         int        `json:"version" default:"2"`
	MainConfig      TestConfig `json:"main_config"`
	SecondaryConfig TestConfig `json:"secondary_config"`
}

// TestSliceConfig represents a configuration with slices
type TestSliceConfig struct {
	Version int          `json:"version" default:"3"`
	Servers []TestConfig `json:"servers"`
}

func TestLoadConfig_Basic(ts *testing.T) {
	// Create a temporary directory for test files
	tempDir := ts.TempDir()
	configPath := filepath.Join(tempDir, "test_config.json")

	// Test 1: Load config with default values (file doesn't exist)
	ts.Run("Load with defaults", func(ts *testing.T) {
		config := &TestConfig{}

		err := LoadConfig(config, 1, configPath, false)
		if err != nil {
			ts.Fatalf("LoadConfig failed: %v", err)
		}

		// Check default values
		if config.Version != 1 {
			ts.Errorf("Expected Version to be 1, got %d", config.Version)
		}
		if config.DatabaseHost != "localhost" {
			ts.Errorf("Expected DatabaseHost to be 'localhost', got '%s'", config.DatabaseHost)
		}
		if config.DatabasePort != 5432 {
			ts.Errorf("Expected DatabasePort to be 5432, got %d", config.DatabasePort)
		}
		if config.DatabaseName != "testdb" {
			ts.Errorf("Expected DatabaseName to be 'testdb', got '%s'", config.DatabaseName)
		}
		if config.DatabaseUser != "testuser" {
			ts.Errorf("Expected DatabaseUser to be 'testuser', got '%s'", config.DatabaseUser)
		}
		if !config.Debug {
			ts.Errorf("Expected Debug to be true, got %v", config.Debug)
		}
	})
}

func TestLoadConfig_WithExistingFile(ts *testing.T) {
	tempDir := ts.TempDir()
	configPath := filepath.Join(tempDir, "test_config.json")

	// Create an existing config file
	existingConfig := &TestConfig{
		Version:          5,
		DatabaseHost:     "existing-host",
		DatabasePort:     8080,
		DatabaseName:     "existing-db",
		DatabaseUser:     "existing-user",
		DatabasePassword: "existing-password",
		Debug:            false,
	}

	// Write the config file
	configData, err := json.MarshalIndent(existingConfig, "", "  ")
	if err != nil {
		ts.Fatalf("Failed to marshal config: %v", err)
	}

	err = os.WriteFile(configPath, configData, 0644)
	if err != nil {
		ts.Fatalf("Failed to write config file: %v", err)
	}

	// Test loading existing config
	ts.Run("Load existing config", func(ts *testing.T) {
		config := &TestConfig{}

		err := LoadConfig(config, 6, configPath, false)
		if err != nil {
			ts.Fatalf("LoadConfig failed: %v", err)
		}

		// Check that values from file are preserved
		if config.DatabaseHost != "existing-host" {
			ts.Errorf("Expected DatabaseHost to be 'existing-host', got '%s'", config.DatabaseHost)
		}
		if config.DatabasePort != 8080 {
			ts.Errorf("Expected DatabasePort to be 8080, got %d", config.DatabasePort)
		}
		if config.DatabaseName != "existing-db" {
			ts.Errorf("Expected DatabaseName to be 'existing-db', got '%s'", config.DatabaseName)
		}
		if config.DatabaseUser != "existing-user" {
			ts.Errorf("Expected DatabaseUser to be 'existing-user', got '%s'", config.DatabaseUser)
		}
		if config.Debug {
			ts.Errorf("Expected Debug to be false, got %v", config.Debug)
		}
		// Check that version was updated
		if config.Version != 6 {
			ts.Errorf("Expected Version to be updated to 6, got %d", config.Version)
		}
	})
}

func TestLoadConfig_PasswordEncryption(ts *testing.T) {
	tempDir := ts.TempDir()
	configPath := filepath.Join(tempDir, "test_config.json")

	ts.Run("Password encryption", func(ts *testing.T) {
		config := &TestConfig{
			DatabasePassword: "plaintext-password",
			APIKey:           "plaintext-api-key",
		}

		// Load config - this should encrypt the passwords
		err := LoadConfig(config, 1, configPath, false)
		if err != nil {
			ts.Fatalf("LoadConfig failed: %v", err)
		}
		// Check that secure password fields are set
		if config.DatabaseSecurePassword == "" {
			ts.Error("DatabaseSecurePassword should be set after encryption")
		}
		if config.APISecureKey != "" {
			ts.Error("APISecureKey should not be set after encryption")
		}

		// Verify that passwords are decrypted and accessible
		if config.DatabasePassword != "plaintext-password" {
			ts.Errorf("Expected decrypted DatabasePassword to be 'plaintext-password', got '%s'", config.DatabasePassword)
		}
		if config.APIKey != "plaintext-api-key" {
			ts.Errorf("Expected decrypted APIKey to be 'plaintext-api-key', got '%s'", config.APIKey)
		}
		configPlain := &TestConfig{}
		fileData, err := os.ReadFile(configPath)
		if err != nil {
			ts.Fatalf("Failed to read config file: %v", err)
		}
		if err := json.Unmarshal(fileData, configPlain); err != nil {
			ts.Fatalf("Failed to unmarshal config file: %v", err)
		}

		// Check that the written values match the expected plaintext values
		expectedValue := t("config.password_message")
		if configPlain.DatabasePassword != expectedValue {
			ts.Errorf("Expected DatabasePassword to be '%s', got '%s'", expectedValue, configPlain.DatabasePassword)
		}
		if configPlain.APIKey != "plaintext-api-key" {
			ts.Errorf("Expected APIKey to be 'plaintext-api-key', got '%s'", configPlain.APIKey)
		}
	})
}

func TestLoadConfig_CleanConfig(ts *testing.T) {
	tempDir := ts.TempDir()
	configPath := filepath.Join(tempDir, "test_config.json")

	ts.Run("Clean config with plaintext passwords", func(ts *testing.T) {
		config := &TestConfig{
			DatabasePassword: "plaintext-password",
			APIKey:           "plaintext-api-key",
		}

		// Load config with cleanConfig=true
		err := LoadConfig(config, 1, configPath, true)
		if err != nil {
			ts.Fatalf("LoadConfig failed: %v", err)
		}

		// Check that passwords remain in plaintext when cleanConfig=true
		if config.DatabasePassword != "plaintext-password" {
			ts.Errorf("Expected DatabasePassword to remain 'plaintext-password', got '%s'", config.DatabasePassword)
		}
		if config.APIKey != "plaintext-api-key" {
			ts.Errorf("Expected APIKey to remain 'plaintext-api-key', got '%s'", config.APIKey)
		}

		// Verify the file was written with plaintext passwords
		fileData, err := os.ReadFile(configPath)
		if err != nil {
			ts.Fatalf("Failed to read config file: %v", err)
		}

		fileContent := string(fileData)
		if !strings.Contains(fileContent, "plaintext-password") {
			ts.Error("Config file should contain plaintext password when cleanConfig=true")
		}
		if !strings.Contains(fileContent, "plaintext-api-key") {
			ts.Error("Config file should contain plaintext API key when cleanConfig=true")
		}
	})
}

func TestLoadConfig_NestedStructures(ts *testing.T) {
	tempDir := ts.TempDir()
	configPath := filepath.Join(tempDir, "nested_config.json")

	ts.Run("Nested configuration structures", func(ts *testing.T) {
		config := &NestedTestConfig{
			MainConfig: TestConfig{
				DatabasePassword: "main-password",
				APIKey:           "main-api-key",
			},
			SecondaryConfig: TestConfig{
				DatabasePassword: "secondary-password",
				APIKey:           "secondary-api-key",
			},
		}

		err := LoadConfig(config, 2, configPath, false)
		if err != nil {
			ts.Fatalf("LoadConfig failed: %v", err)
		}

		// Check that nested passwords are encrypted and decrypted
		if config.MainConfig.DatabasePassword != "main-password" {
			ts.Errorf("Expected MainConfig.DatabasePassword to be 'main-password', got '%s'", config.MainConfig.DatabasePassword)
		}
		if config.SecondaryConfig.DatabasePassword != "secondary-password" {
			ts.Errorf("Expected SecondaryConfig.DatabasePassword to be 'secondary-password', got '%s'", config.SecondaryConfig.DatabasePassword)
		}
		if config.MainConfig.APIKey != "main-api-key" {
			ts.Errorf("Expected MainConfig.APIKey to be 'main-api-key', got '%s'", config.MainConfig.APIKey)
		}
		if config.SecondaryConfig.APIKey != "secondary-api-key" {
			ts.Errorf("Expected SecondaryConfig.APIKey to be 'secondary-api-key', got '%s'", config.SecondaryConfig.APIKey)
		}

		// Check that secure password fields are set
		if config.MainConfig.DatabaseSecurePassword == "" {
			ts.Error("MainConfig.DatabaseSecurePassword should be set")
		}
		if config.SecondaryConfig.DatabaseSecurePassword == "" {
			ts.Error("SecondaryConfig.DatabaseSecurePassword should be set")
		}
	})
}

func TestLoadConfig_SliceStructures(ts *testing.T) {
	tempDir := ts.TempDir()
	configPath := filepath.Join(tempDir, "slice_config.json")

	ts.Run("Configuration with slices", func(ts *testing.T) {
		config := &TestSliceConfig{
			Servers: []TestConfig{
				{
					DatabasePassword: "server1-password",
					APIKey:           "server1-api-key",
				},
				{
					DatabasePassword: "server2-password",
					APIKey:           "server2-api-key",
				},
			},
		}

		err := LoadConfig(config, 3, configPath, false)
		if err != nil {
			ts.Fatalf("LoadConfig failed: %v", err)
		}

		// Check that passwords in slices are encrypted and decrypted
		if config.Servers[0].DatabasePassword != "server1-password" {
			ts.Errorf("Expected Servers[0].DatabasePassword to be 'server1-password', got '%s'", config.Servers[0].DatabasePassword)
		}
		if config.Servers[1].DatabasePassword != "server2-password" {
			ts.Errorf("Expected Servers[1].DatabasePassword to be 'server2-password', got '%s'", config.Servers[1].DatabasePassword)
		}
		if config.Servers[0].APIKey != "server1-api-key" {
			ts.Errorf("Expected Servers[0].APIKey to be 'server1-api-key', got '%s'", config.Servers[0].APIKey)
		}
		if config.Servers[1].APIKey != "server2-api-key" {
			ts.Errorf("Expected Servers[1].APIKey to be 'server2-api-key', got '%s'", config.Servers[1].APIKey)
		}
	})
}

func TestLoadConfig_ErrorCases(ts *testing.T) {
	tempDir := ts.TempDir()

	ts.Run("Invalid config type (not a pointer to struct)", func(ts *testing.T) {
		config := TestConfig{} // Not a pointer

		err := LoadConfig(config, 1, filepath.Join(tempDir, "test.json"), false)
		if err == nil {
			ts.Error("Expected error for non-pointer config, got nil")
		}
		if !contains(err.Error(), t("config.config_no_struct")) {
			ts.Errorf("Expected error message about config not being a struct, got: %v", err)
		}
	})

	ts.Run("Invalid config type (not a struct)", func(ts *testing.T) {
		config := "not a struct"

		err := LoadConfig(&config, 1, filepath.Join(tempDir, "test.json"), false)
		if err == nil {
			ts.Error("Expected error for non-struct config, got nil")
		}
		if !contains(err.Error(), t("config.config_no_struct")) {
			ts.Errorf("Expected error message about config not being a struct, got: %v", err)
		}
	})

	ts.Run("Invalid JSON in config file", func(ts *testing.T) {
		configPath := filepath.Join(tempDir, "invalid.json")

		// Write invalid JSON
		err := os.WriteFile(configPath, []byte(`{"invalid": json}`), 0644)
		if err != nil {
			ts.Fatalf("Failed to write invalid JSON file: %v", err)
		}

		config := &TestConfig{}
		err = LoadConfig(config, 1, configPath, false)
		if err == nil {
			ts.Error("Expected error for invalid JSON, got nil")
		}
		if !contains(err.Error(), t("config.failed_parsing")) {
			ts.Errorf("Expected error message about parsing failure, got: %v", err)
		}
	})
}

func TestLoadConfig_VersionManagement(ts *testing.T) {
	tempDir := ts.TempDir()
	configPath := filepath.Join(tempDir, "version_test.json")

	ts.Run("Version update", func(ts *testing.T) {
		config := &TestConfig{
			Version: 5,
		}

		// Load with new version
		err := LoadConfig(config, 10, configPath, false)
		if err != nil {
			ts.Fatalf("LoadConfig failed: %v", err)
		}

		// Check that version was updated
		if config.Version != 10 {
			ts.Errorf("Expected Version to be updated to 10, got %d", config.Version)
		}
	})

	ts.Run("Version remains same when no change", func(ts *testing.T) {
		config := &TestConfig{
			Version: 10,
		}

		// Load with same version
		err := LoadConfig(config, 10, configPath, false)
		if err != nil {
			ts.Fatalf("LoadConfig failed: %v", err)
		}

		// Check that version remains the same
		if config.Version != 10 {
			ts.Errorf("Expected Version to remain 10, got %d", config.Version)
		}
	})
}

func TestLoadConfig_CustomHardwareID(ts *testing.T) {
	tempDir := ts.TempDir()
	configPath := filepath.Join(tempDir, "custom_hw.json")

	ts.Run("Custom hardware ID function", func(ts *testing.T) {
		config := &TestConfig{
			DatabasePassword: "test-password",
		}

		// Custom hardware ID function that returns a fixed value
		customHardwareID := func() (uint64, error) {
			return 12345, nil
		}

		err := LoadConfig(config, 1, configPath, false, customHardwareID)
		if err != nil {
			ts.Fatalf("LoadConfig failed: %v", err)
		}

		// Verify that password encryption worked with custom hardware ID
		if config.DatabasePassword != "test-password" {
			ts.Errorf("Expected decrypted DatabasePassword to be 'test-password', got '%s'", config.DatabasePassword)
		}
		if config.DatabaseSecurePassword == "" {
			ts.Error("DatabaseSecurePassword should be set after encryption")
		}
	})
}

func TestLoadConfig_FilePermissions(ts *testing.T) {
	tempDir := ts.TempDir()
	configPath := filepath.Join(tempDir, "permissions_test.json")

	ts.Run("File permissions", func(ts *testing.T) {
		config := &TestConfig{
			DatabasePassword: "test-password",
		}

		err := LoadConfig(config, 1, configPath, false)
		if err != nil {
			ts.Fatalf("LoadConfig failed: %v", err)
		}

		// Check that file was created with proper permissions
		fileInfo, err := os.Stat(configPath)
		if err != nil {
			ts.Fatalf("Failed to get file info: %v", err)
		}

		// Check that file is readable and writable by owner
		mode := fileInfo.Mode()
		if mode&0400 == 0 {
			ts.Error("Config file should be readable by owner")
		}
		if mode&0200 == 0 {
			ts.Error("Config file should be writable by owner")
		}
	})
}

// Helper function to check if a string contains a substring that matches to a template
func contains(s, template string) bool {
	if idx := strings.IndexAny(template, "%{"); idx != -1 {
		// There are parameters in the template
		r1 := regexp.MustCompile(`%[-#+ 0]*(?:\*|\d+)?(?:\.(?:\*|\d+))?[hlL]?[diouxXeEfFgGaAcsqpnvtTb%]`)
		template = r1.ReplaceAllString(template, "##PARAM##")
		r1 = regexp.MustCompile(`\{[a-zA-Z_][a-zA-Z0-9_]*(?:,\s*[^}]*)?\}|\{\{[a-zA-Z_][a-zA-Z0-9_.]*\}\}|\{[0-9]+\}`)
		template = r1.ReplaceAllString(template, "##PARAM##")
		template = regexp.QuoteMeta(template)
		template = strings.ReplaceAll(template, "##PARAM##", ".*")
		return regexp.MustCompile(template).MatchString(s)
	} else {
		return strings.Contains(s, template)
	}
}

// Benchmark tests
func BenchmarkLoadConfig_Simple(b *testing.B) {
	tempDir := b.TempDir()
	configPath := filepath.Join(tempDir, "benchmark_config.json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := &TestConfig{
			DatabasePassword: "benchmark-password",
			APIKey:           "benchmark-api-key",
		}

		err := LoadConfig(config, 1, configPath, false)
		if err != nil {
			b.Fatalf("LoadConfig failed: %v", err)
		}
	}
}

func BenchmarkLoadConfig_WithExistingFile(b *testing.B) {
	tempDir := b.TempDir()
	configPath := filepath.Join(tempDir, "benchmark_existing.json")

	// Create existing config file
	existingConfig := &TestConfig{
		Version:          1,
		DatabaseHost:     "localhost",
		DatabasePort:     5432,
		DatabaseName:     "testdb",
		DatabaseUser:     "testuser",
		DatabasePassword: "existing-password",
		Debug:            true,
	}

	configData, _ := json.MarshalIndent(existingConfig, "", "  ")
	os.WriteFile(configPath, configData, 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := &TestConfig{}
		err := LoadConfig(config, 2, configPath, false)
		if err != nil {
			b.Fatalf("LoadConfig failed: %v", err)
		}
	}
}
