// MIT License

// Copyright (c) 2018 Andy Pan

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package ants

import (
	"runtime"
	"time"
)

// goWorker is the actual executor who runs the tasks,
// it starts a goroutine that accepts tasks and
// performs function calls.
type goWorker struct {
	// pool who owns this worker.
	pool *Pool

	// task is a job should be done.
	task chan func()

	// recycleTime will be updated when putting a worker back into queue.
	// 将 worker 放回队列时会更新
	recycleTime time.Time
}

// run starts a goroutine to repeat the process
// that performs the function calls.
// run 启动一个 goroutine 以重复执行函数调用的过程。
func (w *goWorker) run() {
	w.pool.addRunning(1)
	go func() {
		defer func() {
			w.pool.addRunning(-1)
			w.pool.workerCache.Put(w)     // 放回生成器池pool中
			if p := recover(); p != nil { // TODO 捕获任务执行过程中抛出的panic
				// goWorker panic 后的回调
				if ph := w.pool.options.PanicHandler; ph != nil {
					ph(p)
				} else { // 未配置PanicHandler就打印日志
					w.pool.options.Logger.Printf("worker exits from a panic: %v\n", p)
					var buf [4096]byte
					n := runtime.Stack(buf[:], false)
					w.pool.options.Logger.Printf("worker exits from panic: %s\n", string(buf[:n]))
				}
			}
			// Call Signal() here in case there are goroutines waiting for available workers.
			w.pool.cond.Signal()
		}()
		// 实际上for f := range w.task这个循环直到通道task关闭或取出为nil的任务才会终止。
		// 所以这个 goroutine 一直在运行，这正是ants高性能的关键所在。
		// 每个goWorker只会启动一次 goroutine， 后续重复利用这个 goroutine。
		// goroutine 每次只执行一个任务就会被放回池中。
		for f := range w.task { // 消费任务
			if f == nil { // 等待 purgePeriodically和 x.reset() 发送空消息
				return
			}
			f()
			// 如果放回操作失败，则会调用return，这会让 goroutine 运行结束，防止 goroutine 泄漏。
			if ok := w.pool.revertWorker(w); !ok { // TODO 有可能task没有消费完？
				return
			}
		}
	}()
}
