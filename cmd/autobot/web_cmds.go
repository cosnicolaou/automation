// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"cloudeng.io/cmdutil"
	wa "cloudeng.io/webapp/webassets"
	"github.com/cosnicolaou/automation/cmd/autobot/internal/webassets"
)

type WebUIFlags struct {
	Port     string `subcmd:"port,8080,port to listen on"`
	CertFile string `subcmd:"cert,,certificate file"`
	KeyFile  string `subcmd:"key,,key file"`
	Assets   string `subcmd:"assets,,path to assets"`
}

func (fv WebUIFlags) Pages() webassets.Pages {
	rfs := wa.NewAssets("static", webassets.Static,
		wa.EnableReloading(fv.Assets, time.Now(), true))
	return webassets.NewPages(rfs)
}

func (fv WebUIFlags) CreateWebServer(ctx context.Context, mux *http.ServeMux) (*http.Server, func() error, string, error) {
	host := "127.0.0.1"
	tls := fv.CertFile != "" && fv.KeyFile != ""
	if tls {
		host = ""
	}
	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%v", host, fv.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	runner := func() error {
		return server.ListenAndServe()
	}
	url := fmt.Sprintf("http://127.0.0.1:%v", fv.Port)
	if tls {
		runner = func() error {
			return server.ListenAndServeTLS(fv.CertFile, fv.KeyFile)
		}
		url = fmt.Sprintf("https://127.0.0.1:%v", fv.Port)
	}

	cmdutil.HandleSignals(func() {
		_ = server.Shutdown(ctx)
	}, os.Interrupt)

	return server, runner, url, nil
}
