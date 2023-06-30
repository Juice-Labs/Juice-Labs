/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package main

import (
	"context"
	"errors"
	"time"

	"github.com/Juice-Labs/Juice-Labs/internal/backend"
	"github.com/Juice-Labs/Juice-Labs/internal/build"
	"github.com/Juice-Labs/Juice-Labs/pkg/appmain"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

func main() {
	appmain.Run("Juice Controller Backend", build.Version, func(ctx context.Context) error {
		taskManager := task.NewTaskManager(ctx)
		taskManager.GoFn(func(group task.Group) error {
			backend, err := backend.NewBackend(group.Ctx())
			if err != nil {
				return err
			}

			err = backend.Update()
			if err == nil {
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()

			UpdateLoop:
				for {
					select {
					case <-group.Ctx().Done():
						break UpdateLoop

					case <-ticker.C:
						err = backend.Update()
						if err != nil {
							break UpdateLoop
						}
					}
				}
			}

			return errors.Join(err, backend.Close())
		})

		return taskManager.Wait()
	})
}
