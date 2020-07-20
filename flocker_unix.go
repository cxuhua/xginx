// Copyright (c) 2012, Suryandaru Triandana <syndtr@gmail.com>
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// +build darwin dragonfly freebsd linux netbsd openbsd

package xginx

import (
	"os"
	"syscall"
)

type unixFileLock struct {
	path     string
	readOnly bool
	f        *os.File
}

func (fl *unixFileLock) Release() error {
	_ = setFileLock(fl.f, false, false)
	err := fl.f.Close()
	_ = os.Remove(fl.path)
	return err
}

func (fl *unixFileLock) Lock() error {
	var flag int
	if fl.readOnly {
		flag = os.O_RDONLY
	} else {
		flag = os.O_RDWR
	}
	f, err := os.OpenFile(fl.path, flag, 0)
	if os.IsNotExist(err) {
		f, err = os.OpenFile(fl.path, flag|os.O_CREATE, 0644)
	}
	if err != nil {
		return err
	}
	err = setFileLock(f, fl.readOnly, true)
	if err != nil {
		f.Close()
	}
	return err
}

func NewFLocker(path string, readOnly bool) FLocker {
	return &unixFileLock{
		path:     path,
		readOnly: readOnly,
	}
}

func setFileLock(f *os.File, readOnly, lock bool) error {
	how := syscall.LOCK_UN
	if lock {
		if readOnly {
			how = syscall.LOCK_SH
		} else {
			how = syscall.LOCK_EX
		}
	}
	return syscall.Flock(int(f.Fd()), how|syscall.LOCK_NB)
}
