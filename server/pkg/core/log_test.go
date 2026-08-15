package core

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func logConfig(env, level, format string) Config {
	return Config{
		Env: env,
		Log: LogConfig{Level: level, Format: format},
	}
}

func TestTheLogLevelIsReadInAnyCasing(t *testing.T) {
	for _, name := range []string{"debug", "DEBUG", "Debug", " debug "} {
		level, err := ParseLogLevel(name)

		require.NoError(t, err)
		assert.Equal(t, slog.LevelDebug, level)
	}
}

// A config predating the setting keeps the level the server always ran at
func TestAnEmptyLogLevelFallsBackToInfo(t *testing.T) {
	level, err := ParseLogLevel("")

	require.NoError(t, err)
	assert.Equal(t, slog.LevelInfo, level)
}

func TestAnUnknownLogLevelIsRefused(t *testing.T) {
	_, err := ParseLogLevel("chatty")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "chatty")
}

// Auto is what makes the setting worth having a default at all: a terminal in
// development, something a collector parses in production
func TestAutoPicksTheFormatFromTheEnvironment(t *testing.T) {
	prod, err := ResolveLogFormat(logConfig("prod", "info", "auto"))
	require.NoError(t, err)
	assert.Equal(t, LogFormatJSON, prod)

	dev, err := ResolveLogFormat(logConfig("dev", "info", "auto"))
	require.NoError(t, err)
	assert.Equal(t, LogFormatText, dev)
}

func TestAnEmptyFormatBehavesAsAuto(t *testing.T) {
	format, err := ResolveLogFormat(logConfig("prod", "info", ""))

	require.NoError(t, err)
	assert.Equal(t, LogFormatJSON, format)
}

func TestAnExplicitFormatOverridesTheEnvironment(t *testing.T) {
	format, err := ResolveLogFormat(logConfig("prod", "info", "text"))

	require.NoError(t, err)
	assert.Equal(t, LogFormatText, format)
}

func TestAnUnknownLogFormatIsRefused(t *testing.T) {
	_, err := ResolveLogFormat(logConfig("prod", "info", "yaml"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "yaml")
}

// The whole point of the level being configurable: what sits below it never
// reaches the output
func TestTheConfiguredLevelFiltersTheOutput(t *testing.T) {
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })

	var buf bytes.Buffer
	require.NoError(t, setupLogging(logConfig("prod", "warn", "json"), &buf))

	slog.Info("quiet")
	slog.Warn("loud")

	assert.NotContains(t, buf.String(), "quiet")
	assert.Contains(t, buf.String(), "loud")
}

func TestDebugReachesTheOutputWhenItIsAskedFor(t *testing.T) {
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })

	var buf bytes.Buffer
	require.NoError(t, setupLogging(logConfig("dev", "debug", "text"), &buf))

	slog.Debug("detail")

	assert.Contains(t, buf.String(), "detail")
}

func TestSetupLoggingRefusesABadConfig(t *testing.T) {
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })

	var buf bytes.Buffer

	require.Error(t, setupLogging(logConfig("prod", "nonsense", "json"), &buf))
	require.Error(t, setupLogging(logConfig("prod", "info", "nonsense"), &buf))
}
