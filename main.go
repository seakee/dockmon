// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package main wires configuration loading, dependency bootstrap, and process
// lifecycle waiting for the dockmon service.
package main

import (
	"log"
	"os"
	"os/signal"
	"runtime"

	"github.com/seakee/dockmon/app"
	"github.com/seakee/dockmon/bootstrap"
)

// main initializes runtime settings, boots the application, and blocks until
// an OS termination signal arrives.
//
// Returns:
//   - None.
func main() {
	// Use all available CPUs because the service starts concurrent workers.
	runtime.GOMAXPROCS(runtime.NumCPU())

	config, err := app.LoadConfig()
	if err != nil {
		log.Fatal("Loading config error: ", err)
	}

	a, err := bootstrap.NewApp(config)
	if err != nil {
		log.Fatal("New App error: ", err)
	}

	a.Start()

	s := waitForSignal()
	log.Println("Signal received, app closed.", s)
}

// waitForSignal blocks until an interrupt or kill signal is received.
//
// Returns:
//   - os.Signal: the signal that terminates the process.
//
// Example:
//
//	sig := waitForSignal()
//	log.Println("shutdown:", sig)
func waitForSignal() os.Signal {
	signalChan := make(chan os.Signal, 1)
	defer close(signalChan)
	signal.Notify(signalChan, os.Kill, os.Interrupt)
	s := <-signalChan
	signal.Stop(signalChan)
	return s
}
