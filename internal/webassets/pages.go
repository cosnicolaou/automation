// Copyright 2025 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package webassets

import (
	"html/template"
	"io"
)

var (
	testServerIndex *template.Template
	runOpsPage      *template.Template
)

func init() {
	testServerIndex = createTemplateOrDie("static/test-homepage.html")
	runOpsPage = createTemplateOrDie("static/runops.html")
}

func TestPageIndex(w io.Writer, system, controllers, devices, conditions string) error {
	d := struct {
		Name        string
		Controllers template.HTML
		Devices     template.HTML
		Conditions  template.HTML
	}{
		Name:        system,
		Controllers: template.HTML(controllers), //nolint:gosec
		Devices:     template.HTML(devices),     //nolint:gosec
		Conditions:  template.HTML(conditions),  //nolint:gosec
	}
	return testServerIndex.Execute(w, &d)

}

func RunOpsPage(w io.Writer, system, title, table string) error {
	d := struct {
		Name  string
		Title string
		Table template.HTML
	}{
		Name:  system,
		Title: title,
		Table: template.HTML(table), //nolint:gosec
	}
	return runOpsPage.Execute(w, &d)
}
