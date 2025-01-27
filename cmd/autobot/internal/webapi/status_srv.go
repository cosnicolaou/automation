// Copyright 2025 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package webapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"cloudeng.io/datetime"
	"github.com/cosnicolaou/automation/internal/logging"
)

type Status struct {
	l  *slog.Logger
	sr *logging.StatusRecorder
}

func NewStatusServer(l *slog.Logger, sr *logging.StatusRecorder) *Status {
	return &Status{
		l:  l.With("component", "status"),
		sr: sr,
	}
}

type CompletionResponse struct {
	Schedule         string `json:"schedule"`
	Device           string `json:"device"`
	Op               string `json:"op"`
	Date             string `json:"date"`
	Due              string `json:"due"`
	Completed        string `json:"completed"`
	PreConditionCall string `json:"pre_condition_call"`
	Status           string `json:"status"`
	ErrorMessage     string `json:"error_message"`
}

func (s *Status) completed(num int64, recent bool) []CompletionResponse {
	cr := []CompletionResponse{}
	var n int64
	it := s.sr.Completed()
	if recent {
		it = s.sr.CompletedRecent()
	}
	for sr := range it {
		cr = append(cr, CompletionResponse{
			Schedule:         sr.Schedule,
			Device:           sr.Device,
			Op:               sr.Op,
			Date:             fmt.Sprintf("%02d/%02d", sr.Due.Month(), sr.Due.Day()),
			Due:              datetime.TimeOfDayFromTime(sr.Due).String(),
			Completed:        datetime.TimeOfDayFromTime(sr.Completed).String(),
			PreConditionCall: sr.PreConditionCall(),
			Status:           sr.Status(),
			ErrorMessage:     sr.ErrorMessage(),
		})
		n++
		if num > 0 && n >= num {
			break
		}
	}
	return cr
}

func (s *Status) httpError(ctx context.Context, w http.ResponseWriter, u *url.URL, msg, err string, statusCode int) {
	s.l.Log(ctx, slog.LevelInfo, msg, "request", u.String(), "code", statusCode, "error", err)
	http.Error(w, err, http.StatusBadRequest)
}

func (s *Status) ServeCompleted(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	pars := r.URL.Query()
	num, err := strconv.ParseInt(pars.Get("num"), 10, 64)
	if err != nil {
		s.httpError(ctx, w, r.URL, "completed", "invalid num", http.StatusBadRequest)
		num = 0
	}
	order := pars.Get("order")
	recent := order == "recent"
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s.completed(num, recent)); err != nil {
		s.httpError(ctx, w, r.URL, "completed", err.Error(), http.StatusInternalServerError)
	}
}

type PendingResponse struct {
	Schedule         string `json:"schedule"`
	Device           string `json:"device"`
	Op               string `json:"op"`
	Date             string `json:"date"`
	Due              string `json:"due"`
	Pending          string `json:"pending"`
	PreConditionCall string `json:"pre_condition_call"`
}

func (s *Status) pending(num int64) []PendingResponse {
	pr := []PendingResponse{}
	var n int64
	for sr := range s.sr.Pending() {
		pr = append(pr, PendingResponse{
			Schedule: sr.Schedule,
			Device:   sr.Device,
			Op:       sr.Op,
			Date:     fmt.Sprintf("%02d/%02d", sr.Due.Month(), sr.Due.Day()),
			Due:      datetime.TimeOfDayFromTime(sr.Due).String(),
			Pending:  datetime.TimeOfDayFromTime(sr.Due).String(),
		})
		n++
		if num > 0 && n >= num {
			break
		}
	}
	return pr
}

func (s *Status) ServePending(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	pars := r.URL.Query()
	num, err := strconv.ParseInt(pars.Get("num"), 10, 64)
	if err != nil {
		s.httpError(ctx, w, r.URL, "pending", "invalid num", http.StatusBadRequest)
		num = 0
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s.pending(num)); err != nil {
		s.httpError(ctx, w, r.URL, "pending", err.Error(), http.StatusInternalServerError)
	}
}

func AppendStatusAPIEndpoints(ctx context.Context, mux *http.ServeMux, c *Status) {

	mux.HandleFunc("/api/completed", func(w http.ResponseWriter, r *http.Request) {
		c.ServeCompleted(ctx, w, r)
	})
	mux.HandleFunc("/api/pending", func(w http.ResponseWriter, r *http.Request) {
		c.ServePending(ctx, w, r)
	})

}
