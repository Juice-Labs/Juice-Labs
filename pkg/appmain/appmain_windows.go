/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package appmain

import (
	"errors"
	"os"

	"github.com/kolesnikovae/go-winjob"

	"github.com/Xdevlab/Run/pkg/logger"
	pkgWinjob "github.com/Xdevlab/Run/pkg/winjob"
)

type jobObject struct {
	object *winjob.JobObject
}

func (job *jobObject) Close() error {
	if job.object != nil {
		return job.object.Close()
	}

	return nil
}

func newJobObject() closable {
	job, err := pkgWinjob.CreateAnonymous(winjob.WithKillOnJobClose())
	if err == nil {
		var process *os.Process
		process, err = os.FindProcess(os.Getpid())
		if err == nil {
			err = job.Assign(process)
		} else {
			err = errors.Join(err, job.Close())
			job = nil
		}
	}

	if err != nil {
		logger.Error("unable to create job object, ", errors.Join(err, job.Close()))
	}

	return &jobObject{
		object: job,
	}
}
