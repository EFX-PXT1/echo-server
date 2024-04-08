package main

import (
	"log/slog"
	"os"

	"github.com/spf13/viper"
)

func logInit() {
	handlerOptions := &slog.HandlerOptions{
		Level:     slog.Level(viper.GetInt("log.level")),
		AddSource: viper.GetBool("log.source"),
	}

	var logger *slog.Logger
	if viper.GetBool("log.json") {
		jh := slog.NewJSONHandler(os.Stdout, handlerOptions)
		logger = slog.New(jh)
	} else {
		th := slog.NewTextHandler(os.Stdout, handlerOptions)
		logger = slog.New(th)
	}
	slog.SetDefault(logger)
}
