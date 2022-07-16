// Copyright 2019 Andy Pan & Dietoad. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package internal

import (
	"runtime"
	"sync"
	"sync/atomic"
)

type spinLock uint32

const maxBackoff = 16

// 自旋锁在加锁失败之后不会立刻进入等待，而是会继续尝试。这对于很快就能获得锁的应用来说能极大提升性能，因为能避免加锁和解锁导致的线程切换。
// 指数退避，先等 1 个循环周期，通过runtime.Gosched()告诉运行时切换其他 goroutine 运行。
// 如果还是获取不到锁，就再等 2 个周期。如果还是不行，再等 4，8，16…以此类推。这可以防止短时间内获取不到锁，导致 CPU 时间的浪费。
func (sl *spinLock) Lock() {
	backoff := 1
	for !atomic.CompareAndSwapUint32((*uint32)(sl), 0, 1) {
		// Leverage the exponential backoff algorithm, see https://en.wikipedia.org/wiki/Exponential_backoff.
		for i := 0; i < backoff; i++ {
			runtime.Gosched()
		}
		if backoff < maxBackoff {
			backoff <<= 1
		}
	}
}

func (sl *spinLock) Unlock() {
	atomic.StoreUint32((*uint32)(sl), 0)
}

// NewSpinLock instantiates a spin-lock.
func NewSpinLock() sync.Locker {
	return new(spinLock)
}
