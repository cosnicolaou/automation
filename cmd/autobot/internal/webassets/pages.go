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

type Pages struct {
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

func (p *Pages) SetPages(contents map[PageNames]string) {
	p.Lock()
	defer p.Unlock()
	maps.Insert(p.contents, maps.All(contents))
}

func (p *Pages) GetPage(name PageNames) string {
	p.Lock()
	defer p.Unlock()
	return p.contents[name]
}

func NewPages(cfs fs.FS) *Pages {
	return &Pages{cfs: cfs, contents: make(map[PageNames]string)}
}

var (
	testPage = "test-homepage.html"
	opsPage  = "runops.html"
)

func (p *Pages) FS() http.FileSystem {
	return http.FS(p.cfs)
}

func (p *Pages) TestPageHome(w io.Writer, systemfile string) error {
	d := struct {
		Name        string
		Controllers template.HTML
		Devices     template.HTML
		Conditions  template.HTML
	}{
		Name:        systemfile,
		Controllers: template.HTML(p.GetPage(ControllersPage)), //nolint: gosec
		Devices:     template.HTML(p.GetPage(DevicesPage)),     //nolint: gosec
		Conditions:  template.HTML(p.GetPage(ConditionsPage)),  //nolint: gosec
	}

	tpl, err := template.ParseFS(p.cfs, testPage)
	if err != nil {
		return err
	}
	return tpl.Execute(w, &d)
}

func (p *Pages) RunOpsPage(w io.Writer, system, title string, page PageNames) error {
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
