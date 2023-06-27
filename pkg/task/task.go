/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package task

import (
	"context"
	"errors"
	"sync"
)

type TaskFn = func(Group) error

type Task interface {
	Run(group Group) error
}

type Group interface {
	Ctx() context.Context
	Cancel()
	Go(Task)
	GoFn(TaskFn)
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

func (group *TaskManager) Go(task Task) {
	group.waitGroup.Add(1)

	go group.run(task.Run)
}

func (group *TaskManager) GoFn(task TaskFn) {
	group.waitGroup.Add(1)

	go group.run(task)
}

func (group *TaskManager) run(task TaskFn) {
	err := task(group)
	if err != nil {
		group.cancel()
	}

	group.errors <- err
	group.waitGroup.Done()
}
