package internal

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

const (
	// CriticalValue indicates when the low 40 bits are about to run out
	CriticalValue uint64 = (1 << 40) * 8 / 10
	// RenewInterval indicates how often renew retries are performed
	RenewInterval uint64 = 0x01FFFFFFFF
)

// WUID is for internal use only.
type WUID struct {
	sync.Mutex
	Section uint8
	N       uint64
	Tag     string
	Logger  Logger
	Renew   func() error
}

// NewWUID is for internal use only.
func NewWUID(tag string, logger Logger, opts ...Option) *WUID {
	w := &WUID{Tag: tag, Logger: logger}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Next is for internal use only.
func (w *WUID) Next() uint64 {
	x := atomic.AddUint64(&w.N, 1)
	if x&0xFFFFFFFFFF >= CriticalValue && x&RenewInterval == 0 {
		w.Lock()
		renew := w.Renew
		w.Unlock()

		go func() {
			defer func() {
				if r := recover(); r != nil && w.Logger != nil {
					w.Logger.Warn(fmt.Sprintf("[wuid] panic. tag: %s, reason: %+v", w.Tag, r))
				}
			}()

			err := renew()
			if w.Logger == nil {
				return
			}
			if err != nil {
				w.Logger.Warn(fmt.Sprintf("[wuid] renew failed. tag: %s, reason: %s", w.Tag, err.Error()))
			} else {
				w.Logger.Info(fmt.Sprintf("[wuid] renew succeeded. tag: %s", w.Tag))
			}
		}()
	}
	return x
}

// Reset is for internal use only.
func (w *WUID) Reset(n uint64) {
	if w.Section == 0 {
		atomic.StoreUint64(&w.N, n)
	} else {
		atomic.StoreUint64(&w.N, n&0x0FFFFFFFFFFFFFFF|uint64(w.Section)<<60)
	}
}

// VerifyH24 is for internal use only.
func (w *WUID) VerifyH24(h24 uint64) error {
	if h24 == 0 {
		return errors.New("the h24 should not be 0. tag: " + w.Tag)
	}

	if w.Section == 0 {
		if h24 > 0xFFFFFF {
			return errors.New("the h24 should not exceed 0xFFFFFF. tag: " + w.Tag)
		}
	} else {
		if h24 > 0x0FFFFF {
			return errors.New("the h20 should not exceed 0x0FFFFF. tag: " + w.Tag)
		}
	}

	return nil
}

// Logger is for internal use only.
type Logger interface {
	Info(args ...interface{})
	Warn(args ...interface{})
}

// Option is for internal use only.
type Option func(*WUID)

// WithSection is for internal use only.
func WithSection(section uint8) Option {
	if section == 0 || section >= 16 {
		panic("section must be in between [1, 15]")
	}
	return func(w *WUID) {
		w.Section = section
	}
}
