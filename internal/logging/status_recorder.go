// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package logging

import (
	"fmt"
	"iter"
	"strings"
	"sync"
	"time"

	"cloudeng.io/algo/container/list"
)

type StatusRecorder struct {
	mu      sync.Mutex
	done    []*StatusRecord
	waiting *list.Double[*StatusRecord]
}

func NewStatusRecorder() *StatusRecorder {
	return &StatusRecorder{
		done:    make([]*StatusRecord, 0, 1000),
		waiting: list.NewDouble[*StatusRecord](),
	}
}

type StatusRecord struct {
	Schedule         string
	Device           string
	ID               int64 // Unique identifier for this invocation
	Op               string
	OpArgs           []string
	Due              time.Time
	Delay            time.Duration
	PreCondition     string // Name of the precondition, if any
	PreConditionArgs []string

	// The following fields are filled in by the status recorder.
	Pending            time.Time // Time the operation was added to the pending list, set by NewPending
	Completed          time.Time // Time the operation was completed set by Finalize
	PreConditionResult bool      // Set using the argument to Finalize
	Error              error     // Set using the argument to Finalize

	listID list.DoubleID[*StatusRecord]
}

func (sr *StatusRecord) Aborted() bool {
	return sr.PreCondition != "" && !sr.PreConditionResult
}

func (sr *StatusRecord) Status() string {
	if sr.Completed.IsZero() {
		return "pending"
	}
	if sr.Aborted() {
		return "aborted"
	}
	return "completed"
}

func (sr *StatusRecord) Name() string {
	return fmt.Sprintf("%v:%v.%v", sr.Schedule, sr.Device, sr.Op)
}

func (sr *StatusRecord) PreConditionCall() string {
	pre := sr.PreCondition
	if len(sr.PreConditionArgs) > 0 {
		pre += "(" + strings.Join(sr.PreConditionArgs, ", ") + ")"
	}
	return pre
}

func (sr *StatusRecord) ErrorMessage() string {
	if sr.Error == nil {
		return ""
	}
	return sr.Error.Error()
}

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

func (s *StatusRecorder) ResetCompleted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.done = s.done[:0]
}
