package core

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// The formats the server knows how to write
const (
	// LogFormatAuto picks the format from the environment
	LogFormatAuto = "auto"
	LogFormatText = "text"
	LogFormatJSON = "json"
)

// prodEnv is the environment name that turns on the production defaults
const prodEnv = "prod"

// SetupLogging installs the logger described by the config as the slog default
func SetupLogging(config Config) error {
	return setupLogging(config, os.Stderr)
}

func setupLogging(config Config, out io.Writer) error {
	level, err := ParseLogLevel(config.Log.Level)
	if err != nil {
		return err
	}

	format, err := ResolveLogFormat(config)
	if err != nil {
		return err
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if format == LogFormatJSON {
		handler = slog.NewJSONHandler(out, opts)
	} else {
		handler = slog.NewTextHandler(out, opts)
	}

	slog.SetDefault(slog.New(handler))

	return nil
}

// ParseLogLevel reads the configured level, accepting any casing
func ParseLogLevel(name string) (slog.Level, error) {
	name = strings.TrimSpace(name)

	if name == "" {
		return slog.LevelInfo, nil
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(name)); err != nil {
		return 0, fmt.Errorf("log level %q is not one of debug, info, warn or error", name)
	}

	return level, nil
}

func ResolveLogFormat(config Config) (string, error) {
	switch format := strings.ToLower(strings.TrimSpace(config.Log.Format)); format {
	case LogFormatText, LogFormatJSON:
		return format, nil
	case LogFormatAuto, "":
		if strings.EqualFold(strings.TrimSpace(config.Env), prodEnv) {
			return LogFormatJSON, nil
		}

		return LogFormatText, nil
	default:
		return "", fmt.Errorf("log format %q is not one of auto, text or json", config.Log.Format)
	}
}
