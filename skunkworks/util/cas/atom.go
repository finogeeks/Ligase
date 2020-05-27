package cas

import (
	"sync/atomic"
)

var (
	AddInt32              = atomic.AddInt32
	AddInt64              = atomic.AddInt64
	AddUint32             = atomic.AddUint32
	AddUint64             = atomic.AddUint64
	AddUintptr            = atomic.AddUintptr
	CompareAndSwapInt32   = atomic.CompareAndSwapInt32
	CompareAndSwapInt64   = atomic.CompareAndSwapInt64
	CompareAndSwapPointer = atomic.CompareAndSwapPointer
	CompareAndSwapUint32  = atomic.CompareAndSwapUint32
	CompareAndSwapUint64  = atomic.CompareAndSwapUint64
	CompareAndSwapUintptr = atomic.CompareAndSwapUintptr
	LoadInt32             = atomic.LoadInt32
	LoadInt64             = atomic.LoadInt64
	LoadPointer           = atomic.LoadPointer
	LoadUint32            = atomic.LoadUint32
	LoadUint64            = atomic.LoadUint64
	LoadUintptr           = atomic.LoadUintptr
	StoreInt32            = atomic.StoreInt32
	StoreInt64            = atomic.StoreInt64
	StorePointer          = atomic.StorePointer
	StoreUint32           = atomic.StoreUint32
	StoreUint64           = atomic.StoreUint64
	StoreUintptr          = atomic.StoreUintptr
	SwapInt32             = atomic.SwapInt32
	SwapInt64             = atomic.SwapInt64
	SwapPointer           = atomic.SwapPointer
	SwapUint32            = atomic.SwapUint32
	SwapUint64            = atomic.SwapUint64
	SwapUintptr           = atomic.SwapUintptr
)

func LargeAndSetInt32(v int32, p *int32) {
	for {
		v0 := atomic.LoadInt32(p)
		if v > v0 {
			if atomic.CompareAndSwapInt32(p, v0, v) {
				break
			}
		} else {
			break
		}
	}
}

func SmallAndSetInt32(v int32, p *int32) {
	for {
		v0 := atomic.LoadInt32(p)
		if v < v0 {
			if atomic.CompareAndSwapInt32(p, v0, v) {
				break
			}
		} else {
			break
		}
	}
}

func LargeAndSetInt64(v int64, p *int64) {
	for {
		v0 := atomic.LoadInt64(p)
		if v > v0 {
			if atomic.CompareAndSwapInt64(p, v0, v) {
				break
			}
		} else {
			break
		}
	}
}

func SmallAndSetInt64(v int64, p *int64) {
	for {
		v0 := atomic.LoadInt64(p)
		if v < v0 {
			if atomic.CompareAndSwapInt64(p, v0, v) {
				break
			}
		} else {
			break
		}
	}
}
