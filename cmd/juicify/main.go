/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package main

import (
	"fmt"
	"os"

	"github.com/Xdevlab/Run/cmd/internal/build"
	"github.com/Xdevlab/Run/cmd/juicify/app"
	"github.com/Xdevlab/Run/pkg/appmain"
	"github.com/Xdevlab/Run/pkg/sentry"
	"github.com/Xdevlab/Run/pkg/task"
)

func main() {
	name := "run"

	config := appmain.Config{
		Name:    name,
		Version: build.Version,

		SentryConfig: sentry.ClientOptions{
			Dsn:     os.Getenv("JUICE_JUICIFY_SENTRY_DSN"),
			Release: fmt.Sprintf("%s@%s", name, build.Version),
		},
	}

	err := appmain.Run(config, func(group task.Group) error {
		err := app.Run(group)
		group.Cancel()
		return err
	})

	if err != nil {
		os.Exit(appmain.ExitFailure)
	}
}
