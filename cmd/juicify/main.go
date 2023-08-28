/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package main

import (
	"fmt"

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
			Release: fmt.Sprintf("%s@%s", name, build.Version),
			Dsn:     "https://fb7e2006c23d07f7b3ba78067164eba4@o4505739073486848.ingest.sentry.io/4505779685818368",
		},
	}

	appmain.Run(config, func(group task.Group) error {
		err := app.Run(group)
		group.Cancel()
		return err
	})
}
