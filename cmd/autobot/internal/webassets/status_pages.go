// Copyright 2025 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package webassets

import (
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"sync"
	"time"
)

type StatusPages struct {
	sync.Mutex
	cfs     fs.FS
	started time.Time
}

func (p *StatusPages) FS() http.FileSystem {
	return http.FS(p.cfs)
}

func NewStatusPages(cfs fs.FS) *StatusPages {
	return &StatusPages{cfs: cfs, started: time.Now()}
}

const (
	statusPage                    = "status.html"
	statusHomeJS      template.JS = "static/status-home.js"
	statusCompletedJS template.JS = "static/status-completed.js"
	statusPendingJS   template.JS = "static/status-pending.js"
	statusCalendarJS  template.JS = "static/status-calendar.js"
)

type statusData struct {
	Name     string
	DateTime string
	Started  string
	Main     template.HTML
	Script   template.JS
}

func (p *StatusPages) statusData(name string, text template.HTML, script template.JS) statusData {
	return statusData{
		Name:     name,
		DateTime: time.Now().Format(time.RFC1123),
		Started:  p.started.Format(time.RFC1123),
		Main:     text,
		Script:   script,
	}
}

func (p *StatusPages) StatusHomePage(w io.Writer, systemFile string) error {
	d := p.statusData(systemFile, `
		<h2>Completed</h2>
        <div id="completed"></div>
        <h2>Pending</h2>
        <div id="pending"></div>`, statusHomeJS)
	tpl, err := template.ParseFS(p.cfs, statusPage)
	if err != nil {
		return err
	}
	return tpl.Execute(w, &d)
}

func (p *StatusPages) StatusCompletedPage(w io.Writer, systemFile string) error {
	d := p.statusData(systemFile, `
		<h2>Completed</h2>
        <div id="completed"></div>`, statusCompletedJS)
	tpl, err := template.ParseFS(p.cfs, statusPage)
	if err != nil {
		return err
	}
	return tpl.Execute(w, &d)
}

func (p *StatusPages) StatusPendingPage(w io.Writer, systemFile string) error {
	d := p.statusData(systemFile, `
		<h2>Pending</h2>
        <div id="pending"></div>`, statusPendingJS)
	tpl, err := template.ParseFS(p.cfs, statusPage)
	if err != nil {
		return err
	}
	return tpl.Execute(w, &d)
}

func (p *StatusPages) StatusCalendarPage(w io.Writer, systemFile string) error {
	d := p.statusData(systemFile, `
		<h2>Calendar</h2>
		<div id="daterange"></div>
		<div id="schedules"></div>
        <div id="calendar"></div>`, statusCalendarJS)
	tpl, err := template.ParseFS(p.cfs, statusPage)
	if err != nil {
		return err
	}
	return tpl.Execute(w, &d)
}

func AppendStatusPages(mux *http.ServeMux, systemFile string, pages *StatusPages) {
	mux.Handle("/static/",
		http.StripPrefix("/static/", http.FileServer(pages.FS())))

	mux.HandleFunc("/completed", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.StatusCompletedPage(w, systemFile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/pending", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.StatusPendingPage(w, systemFile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.StatusHomePage(w, systemFile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/calendar", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.StatusCalendarPage(w, systemFile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}
