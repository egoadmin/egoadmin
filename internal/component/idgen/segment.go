package idgen

import (
	"math"
	"sync/atomic"
)

type segment struct {
	ptr atomic.Pointer[segmentState]
}

type segmentState struct {
	start  int64
	end    int64
	cursor atomic.Int64
}

func (s *segment) reset(r Range) {
	if r.Empty() {
		s.ptr.Store(nil)
		return
	}
	state := &segmentState{
		start: r.Start,
		end:   r.End,
	}
	state.cursor.Store(r.Start)
	s.ptr.Store(state)
}

func (s *segment) next() (int64, bool) {
	state := s.ptr.Load()
	if state == nil {
		return 0, false
	}
	value := state.cursor.Add(1) - 1
	if value >= state.end {
		return 0, false
	}
	return value, true
}

func (s *segment) reserve(n int64) (Range, bool) {
	if n <= 0 {
		return Range{}, false
	}
	state := s.ptr.Load()
	if state == nil {
		return Range{}, false
	}
	start := state.cursor.Add(n) - n
	end := start + n
	if end < start || end > state.end {
		return Range{}, false
	}
	return Range{Start: start, End: end}, true
}

func (s *segment) remaining() int64 {
	state := s.ptr.Load()
	if state == nil {
		return 0
	}
	cursor := state.cursor.Load()
	if cursor >= state.end {
		return 0
	}
	return state.end - cursor
}

func (s *segment) snapshot() Range {
	state := s.ptr.Load()
	if state == nil {
		return Range{}
	}
	return Range{
		Start: state.start,
		End:   state.end,
	}
}

func (s *segment) len() int64 {
	return s.snapshot().Len()
}

func checkedAdd(start int64, step int64) (int64, error) {
	if start < 0 || step <= 0 {
		return 0, ErrInvalidConfig
	}
	if start > math.MaxInt64-step {
		return 0, ErrOverflow
	}
	return start + step, nil
}
