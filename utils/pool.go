package utils

import (
	"math/bits"
	"sync/atomic"
)

const (
	bufN    = 512
	bufSize = 64 * 1024
	maskN   = (bufN + 63) / 64
)

type paddedMask struct {
	mask atomic.Uint64
	_    [56]byte
}

type Pool struct {
	buf   *[bufN][bufSize]byte
	masks [maskN]paddedMask
}

func NewPool() *Pool {
	return &Pool{buf: new([bufN][bufSize]byte)}
}

func (p *Pool) Get() (int, *[bufSize]byte) {
	const offset = iota
	for i := range maskN {
		mask := &p.masks[(offset+i)%maskN].mask
		reg := mask.Load()
		for reg != ^uint64(0) {
			if bit := bits.TrailingZeros64(^reg); mask.CompareAndSwap(reg, reg|(uint64(1)<<bit)) {
				idx := (i * 64) + bit
				return idx, &p.buf[idx]
			}
		}
	}

	return -1, new([bufSize]byte)
}

func (p *Pool) Put(idx int) {
	if idx != -1 {
		p.masks[idx/64].mask.And(^(uint64(1) << (idx % 64)))
	}
}
