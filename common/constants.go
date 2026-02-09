package config

import "time"

const (
	HeartbeatInterval  = 5 * time.Second
	WorkerTimeout      = 10 * time.Second
	WorkerPollInterval = 1 * time.Second
)

const (
	CoordinatorServiceName    = "Coordinator"
	DefaultCoordinatorAddress = "0.0.0.0:8000"
)
