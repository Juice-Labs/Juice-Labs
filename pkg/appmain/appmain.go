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

	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/sentry"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

type closable interface {
	Close() error
}

type Config struct {
	Name    string
	Version string

	SentryConfig sentry.ClientOptions
}

const (
	ExitSuccess = 0
	ExitFailure = 1
)

var (
	printVersion = flag.Bool("version", false, "Prints the version and exits")
)

func Run(config Config, logic task.TaskFn) {
	flag.Parse()

	var err error

	if *printVersion {
		fmt.Fprintln(os.Stdout, config.Version)
		os.Exit(ExitSuccess)
	}

	if err = sentry.Initialize(config.SentryConfig); err == nil {
		defer sentry.Close()
		err = logger.Configure()
		if err == nil {
			defer logger.Close()
			logger.Info(config.Name, ", v", config.Version)

			// Only available on Windows for cleaning up subprocesses
			job := newJobObject()
			defer job.Close()

			ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

			taskManager := task.NewTaskManager(ctx)
			taskManager.GoFn("AppMain", logic)
			err = taskManager.Wait()
			if err != nil {
				logger.Error(err)
			}
		}
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(ExitFailure)
	}
}
