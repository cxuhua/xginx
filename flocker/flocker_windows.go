// Copyright (c) 2013, Suryandaru Triandana <syndtr@gmail.com>
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package flocker

import (
	"os"
	"syscall"
)

type windowsFileLock struct {
	path     string
	fd       syscall.Handle
	readOnly bool
}

func (fl *windowsFileLock) Release() error {
	_ = os.Remove(fl.path + ".lck")
	return syscall.Close(fl.fd)
}

func (fl *windowsFileLock) Lock() error {
	pathp, err := syscall.UTF16PtrFromString(fl.path)
	if err != nil {
		return err
	}
	var access, shareMode uint32
	if fl.readOnly {
		access = syscall.GENERIC_READ
		shareMode = syscall.FILE_SHARE_READ
	} else {
		access = syscall.GENERIC_READ | syscall.GENERIC_WRITE
	}
	fl.fd, err = syscall.CreateFile(pathp, access, shareMode, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err == syscall.ERROR_FILE_NOT_FOUND {
		fl.fd, err = syscall.CreateFile(pathp, access, shareMode, nil, syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	}
	return err
}

func NewFLocker(path string, readOnly bool) FLocker {
	return &windowsFileLock{
		path:     path,
		readOnly: readOnly,
	}
}
