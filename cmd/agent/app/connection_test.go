/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"context"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/Juice-Labs/Juice-Labs/pkg/errors"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

var (
	defaultConnectionData = restapi.ConnectionData{
		Id:          "test",
		Pid:         "0",
		ProcessName: "test",
	}
)

func TestCancel(t *testing.T) {
	taskManager := task.NewTaskManager(context.Background())

	exitCodeCh := make(chan int)
	defer close(exitCodeCh)

	connection := newConnection(defaultConnectionData, *juicePath, "")
	err := connection.Start(taskManager, exitCodeCh)
	if err == nil {
		ticker := time.NewTicker(2 * time.Second)
		<-ticker.C
		ticker.Stop()

		err = connection.Cancel()
	}

	_, ok := <-exitCodeCh
	if !ok {
		err = errors.Join(err, errors.New("unable to read from channel"))
	}

	if err != nil {
		t.Errorf("failure with, %v", err)
	}

	err = taskManager.Wait()
	if err != nil {
		t.Logf("taskManager.Wait() failed with, %v", err)
	}
}

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}
