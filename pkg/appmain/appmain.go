/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package appmain

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Juice-Labs/pkg/logger"
)

const (
	ExitSuccess = 0
	ExitFailure = 1
)

var (
	printVersion = flag.Bool("version", false, "Prints the version and exits")
)

type AppLogic = func(ctx context.Context) error

func Run(name string, version string, logic AppLogic) {
	flag.Parse()

	if *printVersion {
		fmt.Fprintln(os.Stdout, version)
		os.Exit(ExitSuccess)
	}

	err := logger.Configure()
	if err == nil {
		logger.Info(name, ", v", version)

		ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		err = logic(ctx)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(ExitFailure)
	}
}
