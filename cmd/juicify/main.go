/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package main

import (
	"fmt"
	"os"

	"github.com/Juice-Labs/Juice-Labs/cmd/internal/build"
	"github.com/Juice-Labs/Juice-Labs/cmd/juicify/app"
	"github.com/Juice-Labs/Juice-Labs/pkg/appmain"
	"github.com/Juice-Labs/Juice-Labs/pkg/sentry"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

func main() {
	name := "juicify"

	config := appmain.Config{
		Name:    name,
		Version: build.Version,

		SentryConfig: sentry.ClientOptions{
			Dsn:     os.Getenv("JUICE_JUICIFY_SENTRY_DSN"),
			Release: fmt.Sprintf("%s@%s", name, build.Version),
		},
	}

	appmain.Run(config, func(group task.Group) error {
		err := app.Run(group)
		group.Cancel()
		return err
	})
}
