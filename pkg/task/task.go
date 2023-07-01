/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package task

import (
	"context"
	"errors"
	"sync"

	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
)

type TaskFn = func(Group) error

type Task interface {
	Run(group Group) error
}

type Group interface {
	Ctx() context.Context
	Cancel()
	Go(string, Task)
	GoFn(string, TaskFn)
}

type Waiter interface {
	Wait() error
}

type TaskManager struct {
	ctx    context.Context
	cancel context.CancelFunc

	waitGroup *sync.WaitGroup

	errors chan error
}

func NewTaskManager(ctx context.Context) *TaskManager {
	ctx, cancel := context.WithCancel(ctx)

	return &TaskManager{
		ctx:       ctx,
		cancel:    cancel,
		waitGroup: &sync.WaitGroup{},
		errors:    make(chan error),
	}
}

func (group *TaskManager) Ctx() context.Context {
	return group.ctx
}

func (group *TaskManager) Cancel() {
	group.cancel()
}

func (group *TaskManager) Wait() error {
	resultCh := make(chan error)
	defer close(resultCh)

	go func() {
		var result error

		for err := range group.errors {
			result = errors.Join(result, err)
		}

		resultCh <- result
	}()

	<-group.ctx.Done()
	group.waitGroup.Wait()

	close(group.errors)

	return <-resultCh
}

func (group *TaskManager) Go(label string, task Task) {
	group.waitGroup.Add(1)

	go group.run(label, task.Run)
}

func (group *TaskManager) GoFn(label string, task TaskFn) {
	group.waitGroup.Add(1)

	go group.run(label, task)
}

func (group *TaskManager) run(label string, task TaskFn) {
	logger.Debugf("Task %s starting", label)

	err := task(group)
	if err != nil {
		logger.Debugf("Task %s failed with %s", label, err)
		group.cancel()
	}

	logger.Debugf("Task %s finished", label)

	group.errors <- err
	group.waitGroup.Done()
}
