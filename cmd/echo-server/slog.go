package main

import (
	"log/slog"
	"os"
	"strconv"
)

func logInit() {
	var logSource bool
	var logLevel int

	if os.Getenv("LOG_SOURCE") != "" {
		logSource = true
	}

	if ll := os.Getenv("LOG_LEVEL"); ll != "" {
		if i, err := strconv.Atoi(ll); err == nil {
			logLevel = i
		}
	}

	handlerOptions := &slog.HandlerOptions{
		Level:     slog.Level(logLevel),
		AddSource: logSource,
	}

	var logger *slog.Logger
	if os.Getenv("LOG_JSON") != "" {
		jh := slog.NewJSONHandler(os.Stdout, handlerOptions)
		logger = slog.New(jh)
	} else {
		th := slog.NewTextHandler(os.Stdout, handlerOptions)
		logger = slog.New(th)
	}
	slog.SetDefault(logger)
}
