package telemetry

import "time"

const (
	defaultAPICadence      = 5 * time.Second
	defaultLogsCadence     = 2 * time.Second
	defaultSystemCadence   = 3 * time.Second
	defaultShutdownTimeout = 5 * time.Second
)

type CadenceConfig struct {
	API    time.Duration
	Logs   time.Duration
	System time.Duration
}

type Config struct {
	Cadence         CadenceConfig
	ShutdownTimeout time.Duration
}

func (config Config) WithDefaults() Config {
	if config.Cadence.API <= 0 {
		config.Cadence.API = defaultAPICadence
	}
	if config.Cadence.Logs <= 0 {
		config.Cadence.Logs = defaultLogsCadence
	}
	if config.Cadence.System <= 0 {
		config.Cadence.System = defaultSystemCadence
	}
	if config.ShutdownTimeout <= 0 {
		config.ShutdownTimeout = defaultShutdownTimeout
	}

	return config
}
