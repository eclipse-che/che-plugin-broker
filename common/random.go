//
// Copyright (c) 2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package common

import (
	"math/rand"
	"time"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyz"
const lettersNum = len(letterBytes)

// Random generates random numbers and strings
type Random interface {
	Int(n int) int
	IntFromRange(from int, to int) int
	String(length int) string
}

// TODO docs
// TODO add mocks
type RandomImpl struct {
	rand    *rand.Rand
}

func NewRand() Random {
	return &RandomImpl{
		rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (r *RandomImpl) Int(n int) int {
	return r.rand.Intn(n)
}

func (r *RandomImpl) IntFromRange(from int, to int) int {
	return from + r.rand.Intn(to - from)
}

func (r *RandomImpl) String(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = letterBytes[r.Int(lettersNum)]
	}
	return string(b)
}
