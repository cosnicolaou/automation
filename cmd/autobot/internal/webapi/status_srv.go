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
	"strings"

	"cloudeng.io/datetime"
	"github.com/cosnicolaou/automation/internal/logging"
)

type CalenderGenerator func(schedules []string, dr datetime.CalendarDateRange) (CalendarResponse, error)

type Status struct {
	l      *slog.Logger
	sr     *logging.StatusRecorder
	calGen CalenderGenerator
}

func NewStatusServer(l *slog.Logger, sr *logging.StatusRecorder, calGen CalenderGenerator) *Status {
	return &Status{
		l:      l.With("component", "status"),
		sr:     sr,
		calGen: calGen,
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

type CalendarResponse struct {
	Range     string          `json:"range"`
	Schedules []string        `json:"schedules"`
	Entries   []CalendarEntry `json:"calendar"`
}

type CalendarEntry struct {
	Date      string `json:"date"`
	Time      string `json:"time"`
	Schedule  string `json:"schedule"`
	Device    string `json:"device"`
	Operation string `json:"operation"`
	Condition string `json:"condition"`
}

func decodeCalendarParameters(r *http.Request) (datetime.CalendarDateRange, []string, error) {
	pars := r.URL.Query()
	var from, to datetime.CalendarDate
	if f := pars.Get("from"); f != "" {
		if err := from.Parse(f); err != nil {
			return datetime.CalendarDateRange(0), nil, err
		}
	}
	if t := pars.Get("to"); t != "" {
		if err := to.Parse(t); err != nil {
			return datetime.CalendarDateRange(0), nil, err
		}
	}
	cals := strings.Split(pars.Get("calendar"), ",")
	return datetime.NewCalendarDateRange(from, to), cals, nil
}

func (s *Status) ServeCalendar(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	dr, scheds, err := decodeCalendarParameters(r)
	if err != nil {
		s.httpError(ctx, w, r.URL, "calendar", err.Error(), http.StatusBadRequest)
		return
	}
	cr, err := s.calGen(scheds, dr)
	if err != nil {
		s.httpError(ctx, w, r.URL, "calendar", err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cr); err != nil {
		s.httpError(ctx, w, r.URL, "calendar", err.Error(), http.StatusInternalServerError)
	}
}

func (s *Status) AppendEndpoints(ctx context.Context, mux *http.ServeMux) {
	mux.HandleFunc("/api/completed", func(w http.ResponseWriter, r *http.Request) {
		s.ServeCompleted(ctx, w, r)
	})
	mux.HandleFunc("/api/pending", func(w http.ResponseWriter, r *http.Request) {
		s.ServePending(ctx, w, r)
	})
	mux.HandleFunc("/api/calendar", func(w http.ResponseWriter, r *http.Request) {
		s.ServeCalendar(ctx, w, r)
	})
}
