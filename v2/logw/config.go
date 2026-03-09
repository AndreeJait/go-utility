package logw

import "io"

// LogFormat defines the output format of the log.
type LogFormat string

const (
	// FormatJSON outputs the log in JSON format.
	FormatJSON LogFormat = "JSON"
	// FormatText outputs the log in plain text format.
	FormatText LogFormat = "TEXT"
)

// LogConfig holds the configuration for initializing the global logger.
type LogConfig struct {
	Level  string    // Level specifies the minimum log level (debug, info, warn, error).
	Format LogFormat // Format specifies the log output format (default is JSON).

	// File Configuration
	WriteToFile bool
	FilePath    string // FilePath specifies the location of the log file (e.g., "/var/log/app/service.log").

	// Message Broker Configuration
	SendToBroker bool
	BrokerWriter io.Writer // BrokerWriter is an agnostic adapter (e.g., Kafka/NSQ) that implements io.Writer.
}
