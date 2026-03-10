// Package configw provides a robust configuration loader.
// It supports loading configurations from YAML files, Dotenv (.env) files,
// and automatically overrides them with OS Environment Variables.
package configw

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Load reads a configuration file from the given path and unmarshals it into the target struct.
// The target must be a pointer to a struct holding your configuration fields.
//
// Supported formats: .yaml, .yml, .env
func Load(filePath string, target any) error {
	v := viper.New()

	// 1. Set the file path
	v.SetConfigFile(filePath)

	// 2. Enable automatic OS Environment Variable overrides.
	// E.g., if you have a struct field `Database.Host`, an OS env var `DATABASE_HOST` will override it.
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 3. Read the file
	if err := v.ReadInConfig(); err != nil {
		// If it's a .env file that wasn't found, you might want to allow it to fail gracefully
		// if you rely entirely on OS env vars in production. But for strictness, we return the error.
		return fmt.Errorf("configw: failed to read config file '%s': %w", filePath, err)
	}

	// 4. Unmarshal into the provided struct
	// Viper uses the `mapstructure` tag to map fields.
	if err := v.Unmarshal(target); err != nil {
		return fmt.Errorf("configw: failed to unmarshal config into struct: %w", err)
	}

	return nil
}

// LoadEnv loads environment variables directly from a .env file into OS environment variables.
// This is a lightweight alternative if you don't want to unmarshal into a struct.
func LoadEnv(filePath string) error {
	v := viper.New()
	v.SetConfigFile(filePath)
	v.SetConfigType("env")

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("configw: failed to read .env file '%s': %w", filePath, err)
	}

	// Range over the env file and set them to the OS environment so os.Getenv() works.
	for key, value := range v.AllSettings() {
		// Viper lowercases keys by default, so we format them back to standard ENV format.
		// NOTE: viper.BindEnv is typically used, but for raw loading, we set them manually.
		viper.SetDefault(strings.ToUpper(key), value)
	}

	return nil
}
