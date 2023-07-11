/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package appmain

import (
	"github.com/kolesnikovae/go-winjob"
)

type jobObject struct{}

func (job *jobObject) Close() error {
	return nil
}

func newJobObject() closable {
	return &jobObject{}
}
