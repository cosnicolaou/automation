// Copyright 2025 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package webapi

import (
	"net/http"

	"github.com/cosnicolaou/automation/autobot/internal/webassets"
)

func AppendTestServerEndpoints(mux *http.ServeMux,
	cfg string,
	controllersTable string,
	devicesTable string,
	conditionsTable string,
	controllers string,
	devices string,
	conditions string,
) {
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/index.html", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/index.html", func(w http.ResponseWriter, _ *http.Request) {
		err := webassets.TestPageIndex(w, cfg, controllersTable, devicesTable, conditionsTable)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/controllers", func(w http.ResponseWriter, _ *http.Request) {
		err := webassets.RunOpsPage(w, cfg, "controller operations", controllers)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/devices", func(w http.ResponseWriter, _ *http.Request) {
		err := webassets.RunOpsPage(w, cfg, "device operations", devices)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/conditions", func(w http.ResponseWriter, _ *http.Request) {
		err := webassets.RunOpsPage(w, cfg, "device conditions", conditions)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

}
