// Package main is the entry point for the ThreadGate Standalone OTBR Orchestrator.
package main

import (
	"log"
	"os"

	"github.com/MikeO7/threadgate/src/manager/internal/app"
)

type appRunner interface {
	Run() error
}

var (
	newAppRunner = func() appRunner { return app.New() }
	exitFunc     = os.Exit
)

func run() error {
	return newAppRunner().Run()
}

func main() {
	if err := run(); err != nil {
		log.Printf("[Main] Application failed critically: %v", err)
		exitFunc(1)
	}
}
