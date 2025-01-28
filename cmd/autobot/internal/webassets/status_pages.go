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
)

type StatusPages struct {
	sync.Mutex
	cfs fs.FS
}

func (p *StatusPages) FS() http.FileSystem {
	return http.FS(p.cfs)
}

func NewStatusPages(cfs fs.FS) *StatusPages {
	return &StatusPages{cfs: cfs}
}

func (p *StatusPages) StatusHomePage(w io.Writer, systemfile string) error {
	d := struct {
		Name   string
		Main   template.HTML
		Script template.JS
	}{
		Name: systemfile,
		Main: `
		<h2>Completed</h2>
        <div id="completed"></div>
        <h2>Pending</h2>
        <div id="pending"></div>`,
		Script: "static/status-home.js",
	}
	tpl, err := template.ParseFS(p.cfs, statusPage)
	if err != nil {
		return err
	}
	return tpl.Execute(w, &d)
}

func (p *StatusPages) StatusCompletedPage(w io.Writer, systemfile string) error {
	d := struct {
		Name   string
		Main   template.HTML
		Script template.JS
	}{
		Name: systemfile,
		Main: `
		<h2>Completed</h2>
        <div id="completed"></div>`,
		Script: "static/status-completed.js",
	}
	tpl, err := template.ParseFS(p.cfs, statusPage)
	if err != nil {
		return err
	}
	return tpl.Execute(w, &d)
}

func (p *StatusPages) StatusPendingPage(w io.Writer, systemfile string) error {
	d := struct {
		Name   string
		Main   template.HTML
		Script template.JS
	}{
		Name: systemfile,
		Main: `
		<h2>Pending</h2>
        <div id="pending"></div>`,
		Script: "static/status-pending.js",
	}
	tpl, err := template.ParseFS(p.cfs, statusPage)
	if err != nil {
		return err
	}
	return tpl.Execute(w, &d)
}

func (p *StatusPages) StatusCalendarPage(w io.Writer, systemfile string) error {
	d := struct {
		Name   string
		Main   template.HTML
		Script template.JS
	}{
		Name: systemfile,
		Main: `
		<h2>Calendar</h2>
		<div id="daterange"></div>
		<div id="schedules"></div>
        <div id="calendar"></div>`,
		Script: "static/status-calendar.js",
	}
	tpl, err := template.ParseFS(p.cfs, statusPage)
	if err != nil {
		return err
	}
	return tpl.Execute(w, &d)
}

func AppendStatusPages(mux *http.ServeMux, systemfile string, pages *StatusPages) {
	mux.Handle("/static/",
		http.StripPrefix("/static/", http.FileServer(pages.FS())))

	mux.HandleFunc("/completed", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.StatusCompletedPage(w, systemfile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/pending", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.StatusPendingPage(w, systemfile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.StatusHomePage(w, systemfile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/calendar", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.StatusCalendarPage(w, systemfile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}
