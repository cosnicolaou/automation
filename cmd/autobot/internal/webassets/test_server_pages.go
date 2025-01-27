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
	testPage   = "test-server-home.html"
	opsPage    = "test-server-ops.html"
	statusPage = "status.html"
)

func (p *TestServerPages) FS() http.FileSystem {
	return http.FS(p.cfs)
}

func (p *TestServerPages) TestPageHome(w io.Writer, systemfile string) error {
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
		Script:      "static/test-homepage.js",
	}

	tpl, err := template.ParseFS(p.cfs, testPage)
	if err != nil {
		return err
	}
	return tpl.Execute(w, &d)
}

func (p *TestServerPages) RunOpsPage(w io.Writer, system, title string, page PageNames) error {
	d := struct {
		Name  string
		Title string
		Table template.HTML
	}{
		Name:  system,
		Title: title,
		Table: template.HTML(p.GetPage(page)), //nolint: gosec
	}
	tpl, err := template.ParseFS(p.cfs, opsPage)
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

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.TestPageHome(w, systemfile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}
