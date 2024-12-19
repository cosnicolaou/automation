// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package internal

import (
	"iter"
	"sync"
	"time"

	"cloudeng.io/algo/container/list"
	"cloudeng.io/datetime"
)

type StatusRecorder struct {
	mu      sync.Mutex
	counter int64
	done    []*StatusRecord
	waiting *list.Double[*StatusRecord]
	date    datetime.CalendarDate
}

func NewStatusRecorder() *StatusRecorder {
	return &StatusRecorder{
		done:    make([]*StatusRecord, 0, 1000),
		waiting: list.NewDouble[*StatusRecord](),
	}
}

type StatusRecord struct {
	Schedule     string
	Device       string
	ID           int64 // Unique identifier for this invocation
	Op           string
	Due          time.Time
	Delay        time.Duration
	PreCondition string // Name of the precondition, if any

	// The following fields are filled in by the status recorder.
	Pending            time.Time // Time the operation was added to the pending list, set by NewPending
	Completed          time.Time // Time the operation was completed set by Finalize
	PreConditionResult bool      // Set using the argument to Finalize
	Error              error     // Set using the argument to Finalize

	listID list.DoubleID[*StatusRecord]
}

// Need a flush/reset option

func (s *StatusRecorder) PendingDone(sr *StatusRecord, precondition bool, err error) {
	if sr == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sr.Completed = time.Now().In(sr.Due.Location())
	sr.PreConditionResult = precondition
	sr.Error = err
	s.done = append(s.done, sr)
	s.waiting.RemoveItem(sr.listID)
}

func (s *StatusRecorder) NewPending(sr *StatusRecord) *StatusRecord {
	if sr == nil {
		return sr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sr.listID = s.waiting.Append(sr)
	sr.Pending = time.Now().In(sr.Due.Location())
	return sr
}

func (s *StatusRecorder) Completed() iter.Seq[*StatusRecord] {
	return func(yield func(*StatusRecord) bool) {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, sr := range s.done {
			if !yield(sr) {
				return
			}
		}
	}
}

func (s *StatusRecorder) Pending() iter.Seq[*StatusRecord] {
	return func(yield func(*StatusRecord) bool) {
		s.mu.Lock()
		defer s.mu.Unlock()
		for sr := range s.waiting.Forward() {
			if !yield(sr) {
				return
			}
		}
	}
}
