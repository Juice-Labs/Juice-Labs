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
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

type closable interface {
	Close() error
}

const (
	ExitSuccess = 0
	ExitFailure = 1
)

var (
	printVersion = flag.Bool("version", false, "Prints the version and exits")
)

func Run(name string, version string, logic task.TaskFn) {
	flag.Parse()

	if *printVersion {
		fmt.Fprintln(os.Stdout, version)
		os.Exit(ExitSuccess)
	}

	err := logger.Configure()
	if err == nil {
		logger.Info(name, ", v", version)

		// Only available on Windows for cleaning up subprocesses
		job := newJobObject()
		defer job.Close()

		ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

		taskManager := task.NewTaskManager(ctx)
		taskManager.GoFn("AppMain", logic)
		err = taskManager.Wait()
	}

	if err != nil {
		logger.Fatal(err)
	}
}
