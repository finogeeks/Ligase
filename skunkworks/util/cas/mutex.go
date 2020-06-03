package cas

import (
	"sync/atomic"
	"time"
)

const DefLoopTime = time.Millisecond * 5

type Mutex struct {
	LoopTimeout time.Duration
	isLock      int32
}

func (m *Mutex) Lock() {
	timeout := m.LoopTimeout
	if timeout <= 0 {
		timeout = DefLoopTime
	}
	for {
		if m.isLock == 0 && atomic.CompareAndSwapInt32(&m.isLock, 0, 1) {
			break
		}
		time.Sleep(timeout)
	}
}

func (m *Mutex) Unlock() {
	m.isLock = 0
}

// type RWMutex struct {
// 	w           Mutex  // held if there are pending writers
// 	writerSem   uint32 // semaphore for writers to wait for completing readers
// 	readerSem   uint32 // semaphore for readers to wait for completing writers
// 	readerCount int32  // number of pending readers
// 	readerWait  int32  // number of departing readers
// }

// func (m *RWMutex) Lock() {
// 	m.w.Lock()
// }

// func (m *RWMutex) Unlock() {

// }

// func (m *RWMutex) RLock() {

// }

// func (m *RWMutex) RUnlock() {

// }
