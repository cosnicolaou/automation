// Copyright 2025 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package webassets

import (
	"html/template"
	"io"
	"io/fs"
	"net/http"
)

type Pages struct {
	cfs fs.FS
}

func NewPages(cfs fs.FS) Pages {
	return Pages{cfs: cfs}
}

var (
	testPage = "test-homepage.html"
	opsPage  = "runops.html"
)

func (p Pages) FS() http.FileSystem {
	return http.FS(p.cfs)
}

func (p Pages) TestPageIndex(w io.Writer, system, controllers, devices, conditions string) error {
	d := struct {
		Name        string
		Controllers template.HTML
		Devices     template.HTML
		Conditions  template.HTML
	}{
		Name:        system,
		Controllers: template.HTML(controllers), //nolint: gosec
		Devices:     template.HTML(devices),     //nolint: gosec
		Conditions:  template.HTML(conditions),  //nolint: gosec
	}

	/*_, err := readContents(p.cfs, testPage)
	if err != nil {
		fmt.Printf("failed to read contents: %v, %v\n", testPage, err)
	}*/
	tpl, err := template.ParseFS(p.cfs, testPage)
	if err != nil {
		return err
	}
	return tpl.Execute(w, &d)
}

func (p Pages) RunOpsPage(w io.Writer, system, title, table string) error {
	d := struct {
		Name  string
		Title string
		Table template.HTML
	}{
		Name:  system,
		Title: title,
		Table: template.HTML(table), //nolint: gosec
	}
	tpl, err := template.ParseFS(p.cfs, opsPage)
	if err != nil {
		return err
	}
	return tpl.Execute(w, &d)
}
