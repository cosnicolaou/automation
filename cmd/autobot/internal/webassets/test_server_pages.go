// Copyright 2025 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package webassets

import (
	"html/template"
	"io"
	"io/fs"
	"maps"
	"net/http"
	"sync"
)

type TestServerPages struct {
	sync.Mutex
	cfs      fs.FS
	contents map[PageNames]string
}

type PageNames int

const (
	ControllersPage PageNames = iota
	DevicesPage
	ConditionsPage
	ControllerOperationsPage
	DeviceOperationsPage
	DeviceConditionsPage
	ConditionalOperationsPage
)

func (p *TestServerPages) SetPages(contents map[PageNames]string) {
	p.Lock()
	defer p.Unlock()
	maps.Insert(p.contents, maps.All(contents))
}

func (p *TestServerPages) GetPage(name PageNames) string {
	p.Lock()
	defer p.Unlock()
	return p.contents[name]
}

func NewTestServerPages(cfs fs.FS) *TestServerPages {
	return &TestServerPages{cfs: cfs, contents: make(map[PageNames]string)}
}

var (
	testServerHomePage = "test-server-home.html"
	testServerOpsPage  = "test-server-ops.html"

	testServerHomeJS template.JS = "static/test-server-home.js"
)

func (p *TestServerPages) FS() http.FileSystem {
	return http.FS(p.cfs)
}

func (p *TestServerPages) TestServerHomePage(w io.Writer, systemfile string) error {
	d := struct {
		Name        string
		Controllers template.HTML
		Devices     template.HTML
		Conditions  template.HTML
		Script      template.JS
	}{
		Name:        systemfile,
		Controllers: template.HTML(p.GetPage(ControllersPage)), //nolint: gosec
		Devices:     template.HTML(p.GetPage(DevicesPage)),     //nolint: gosec
		Conditions:  template.HTML(p.GetPage(ConditionsPage)),  //nolint: gosec
		Script:      testServerHomeJS,
	}

	tpl, err := template.ParseFS(p.cfs, testServerHomePage)
	if err != nil {
		return err
	}
	return tpl.Execute(w, &d)
}

func (p *TestServerPages) RunOpsPage(w io.Writer, system, title string, page PageNames) error {
	d := struct {
		Name   string
		Title  string
		Table  template.HTML
		Script template.JS
	}{
		Name:   system,
		Title:  title,
		Table:  template.HTML(p.GetPage(page)), //nolint: gosec
		Script: testServerHomeJS,
	}
	tpl, err := template.ParseFS(p.cfs, testServerOpsPage)
	if err != nil {
		return err
	}
	return tpl.Execute(w, &d)
}

func AppendTestServerPages(mux *http.ServeMux,
	systemfile string,
	pages *TestServerPages,
) {

	mux.Handle("/static/",
		http.StripPrefix("/static/", http.FileServer(pages.FS())))

	mux.HandleFunc("/controllers", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.RunOpsPage(w, systemfile, "controller operations", ControllerOperationsPage)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/devices", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.RunOpsPage(w, systemfile, "device operations", DeviceOperationsPage)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/conditions", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.RunOpsPage(w, systemfile, "device conditions", DeviceConditionsPage)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/conditionally", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.RunOpsPage(w, systemfile, "conditional device operations", ConditionalOperationsPage)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.TestServerHomePage(w, systemfile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}
