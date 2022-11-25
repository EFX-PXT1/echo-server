//go:build !windows

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// create a context which is cancelled on termination signals
func signalContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, os.Interrupt, syscall.SIGTERM, syscall.SIGUSR1)
	go func() {
		<-signalC
		cancel()
	}()
	return ctx
}
