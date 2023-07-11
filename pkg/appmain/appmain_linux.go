/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package appmain

type jobObject struct{}

func (job *jobObject) Close() error {
	return nil
}

func newJobObject() closable {
	return &jobObject{}
}
