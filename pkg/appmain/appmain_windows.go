/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package appmain

import (
	"errors"
	"os"

	"github.com/kolesnikovae/go-winjob"

	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
)

type jobObject struct {
	object *winjob.JobObject
}

func (job *jobObject) Close() error {
	return job.object.Close()
}

func newJobObject() closable {
	job, err := winjob.Create("Juicify", winjob.WithKillOnJobClose())
	if err == nil {
		process, err_ := os.FindProcess(os.Getpid())
		err = err_
		if err == nil {
			err = errors.Join(err, job.Assign(process))
		} else {
			err = errors.Join(err, job.Close())
			job = nil
		}
	}

	if err != nil {
		logger.Error("unable to create job object, ", err)
	}

	return &jobObject{
		object: job,
	}
}
