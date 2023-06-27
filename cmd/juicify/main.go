/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package main

import (
	"context"

	"github.com/Juice-Labs/Juice-Labs/cmd/build"
	"github.com/Juice-Labs/Juice-Labs/cmd/juicify/app"
	"github.com/Juice-Labs/Juice-Labs/pkg/appmain"
)

func main() {
	appmain.Run("juicify", build.Version, func(ctx context.Context) error {
		return app.Run(ctx)
	})
}
