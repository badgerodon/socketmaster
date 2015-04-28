package server

import (
	"log"
	"os"
	"time"
)

type (
	Config struct {
		// MissingRouteTimeout is the amount of time to keep trying to find a
		// downstream connection for an upstream connection
		MissingRouteTimeout time.Duration
		// EmptyListenerTimeout is the amount of time to keep an existing upstream
		// listener open
		EmptyListenerTimeout time.Duration
		Logger               *log.Logger
	}
)

func DefaultConfig() *Config {
	logger := log.New(os.Stderr, "[socketmaster] ", 0)
	return &Config{
		MissingRouteTimeout:  time.Second * 30,
		EmptyListenerTimeout: time.Second * 30,
		Logger:               logger,
	}
}
