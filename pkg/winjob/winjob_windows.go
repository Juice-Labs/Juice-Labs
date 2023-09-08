/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package winjob

import (
	"os"
	"syscall"
	"unsafe"

	"github.com/kolesnikovae/go-winjob"
	"github.com/kolesnikovae/go-winjob/jobapi"
)

var (
	modKernel32     = syscall.NewLazyDLL("kernel32.dll")
	createJobObject = modKernel32.NewProc("CreateJobObjectW")
)

// CreateJobObject creates or opens a job object.
//
// https://docs.microsoft.com/en-us/windows/desktop/api/jobapi2/nf-jobapi2-createjobobjectw
func CreateAnonymousJobObject(sa *syscall.SecurityAttributes) (syscall.Handle, error) {
	h, _, lastErr := createJobObject.Call(
		uintptr(unsafe.Pointer(sa)),
		uintptr(0))
	if h == 0 {
		return syscall.InvalidHandle, os.NewSyscallError("CreateAnonymousJobObject", lastErr)
	}
	return syscall.Handle(h), nil
}

func CreateAnonymous(limits ...winjob.Limit) (*winjob.JobObject, error) {
	hJobObject, err := CreateAnonymousJobObject(jobapi.MakeSA())
	if err != nil {
		return nil, err
	}
	job := winjob.JobObject{
		Handle: hJobObject,
	}
	if len(limits) != 0 {
		if err := job.SetLimit(limits...); err != nil {
			_ = job.Close()
			return nil, err
		}
	}
	return &job, nil
}
