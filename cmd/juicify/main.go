/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package main

import (
	"github.com/Juice-Labs/Juice-Labs/cmd/internal/build"
	"github.com/Juice-Labs/Juice-Labs/cmd/juicify/app"
	"github.com/Juice-Labs/Juice-Labs/pkg/appmain"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

func main() {
	appmain.Run("juicify", build.Version, func(group task.Group) error {
		err := app.Run(group)
		group.Cancel()
		if err != nil {
			logger.Fatal(err)
		}
		return err
	})
}
