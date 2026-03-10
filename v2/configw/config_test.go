package configw

import (
	"os"
	"path/filepath"
	"testing"
)

// AppConfig is the target struct we will unmarshal our files into.
// Notice we use the `mapstructure` tag for Viper.
type AppConfig struct {
	AppName string `mapstructure:"APP_NAME"`
	Port    int    `mapstructure:"PORT"`
	DB      struct {
		Host string `mapstructure:"HOST"`
		User string `mapstructure:"USER"`
	} `mapstructure:"DB"`
}

func TestLoad_YAML(t *testing.T) {
	// 1. Create a temporary YAML file
	yamlContent := []byte(`
APP_NAME: "HexagonalApp"
PORT: 8080
DB:
  HOST: "localhost"
  USER: "root"
`)
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(filePath, yamlContent, 0644); err != nil {
		t.Fatalf("Failed to write temp yaml: %v", err)
	}

	// 2. Load it
	var cfg AppConfig
	if err := Load(filePath, &cfg); err != nil {
		t.Fatalf("Expected Load to succeed, got: %v", err)
	}

	// 3. Verify
	if cfg.AppName != "HexagonalApp" || cfg.Port != 8080 || cfg.DB.Host != "localhost" {
		t.Errorf("YAML parsing failed. Got: %+v", cfg)
	}
}

func TestLoad_DotEnv(t *testing.T) {
	// 1. Create a temporary .env file
	envContent := []byte(`
APP_NAME="DotEnvApp"
PORT=9090
DB.HOST="127.0.0.1"
DB.USER="admin"
`)
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, ".dev.env")
	if err := os.WriteFile(filePath, envContent, 0644); err != nil {
		t.Fatalf("Failed to write temp env: %v", err)
	}

	// 2. Load it
	var cfg AppConfig
	if err := Load(filePath, &cfg); err != nil {
		t.Fatalf("Expected Load to succeed, got: %v", err)
	}

	// 3. Verify
	if cfg.AppName != "DotEnvApp" || cfg.Port != 9090 || cfg.DB.Host != "127.0.0.1" {
		t.Errorf("DotEnv parsing failed. Got: %+v", cfg)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	yamlContent := []byte(`
APP_NAME: "OriginalApp"
PORT: 8080
`)
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.yaml")
	_ = os.WriteFile(filePath, yamlContent, 0644)

	// Simulate a Docker Environment Variable injection overriding the YAML file
	os.Setenv("PORT", "9999")
	defer os.Unsetenv("PORT")

	var cfg AppConfig
	_ = Load(filePath, &cfg)

	// Verify the OS environment variable took precedence over the YAML file
	if cfg.Port != 9999 {
		t.Errorf("Expected PORT to be overridden to 9999, got: %d", cfg.Port)
	}
}
