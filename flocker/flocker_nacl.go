// Copyright (c) 2012, Suryandaru Triandana <syndtr@gmail.com>
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// +build nacl

package flocker

import (
	"syscall"
)

type fs struct {
}

func (f *fs) Lock() error {
	return syscall.ENOTSUP
}

func (f *fs) Release() error {
	return syscall.ENOTSUP
}

func NewFLocker(path string, readOnly bool) FLocker {
	return &fs{}
}
