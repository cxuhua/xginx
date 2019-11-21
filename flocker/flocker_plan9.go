// Copyright (c) 2012, Suryandaru Triandana <syndtr@gmail.com>
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package flocker

import (
	"os"
)

type plan9FileLock struct {
	path     string
	readOnly bool
	f        *os.File
}

func (fl *plan9FileLock) Release() error {
	return fl.f.Close()
}

func (fl *plan9FileLock) Lock() error {
	var (
		flag int
		perm os.FileMode
	)
	if fl.readOnly {
		flag = os.O_RDONLY
	} else {
		flag = os.O_RDWR
		perm = os.ModeExclusive
	}
	f, err := os.OpenFile(fl.path, flag, perm)
	if os.IsNotExist(err) {
		f, err = os.OpenFile(fl.path, flag|os.O_CREATE, perm|0644)
	}
	return err
}

func NewFLocker(path string, readOnly bool) FLocker {
	return &plan9FileLock{
		path:     path,
		readOnly: readOnly,
	}
}
