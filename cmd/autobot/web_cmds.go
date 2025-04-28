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
	"cloudeng.io/logging/ctxlog"
	"cloudeng.io/sync/errgroup"
	wa "cloudeng.io/webapp/webassets"
	"github.com/cosnicolaou/automation/cmd/autobot/internal/webassets"
)

type WebUIFlags struct {
	HTTPSRedirectAddr string `subcmd:"https-redirect,127.0.0.1:8083,redirect from http to this https address"`
	HTTPSAddr         string `subcmd:"https-addr,127.0.0.1:8083,https address to listen on"`
	HTTPAddr          string `subcmd:"http-addr,127.0.0.1:8080,http address to listen on"`
	CertFile          string `subcmd:"ssl-cert,,certificate file"`
	KeyFile           string `subcmd:"ssl-key,,key file"`
	Assets            string `subcmd:"web-assets,,path to assets"`
}

func (fv WebUIFlags) TestServerPages() *webassets.TestServerPages {
	rfs := wa.NewAssets("static", webassets.Static,
		wa.EnableReloading(fv.Assets, time.Now(), true))
	return webassets.NewTestServerPages(rfs)
}

func (fv WebUIFlags) StatusPages() *webassets.StatusPages {
	rfs := wa.NewAssets("static", webassets.Static,
		wa.EnableReloading(fv.Assets, time.Now(), true))
	return webassets.NewStatusPages(rfs)
}

func (fv WebUIFlags) createTLSServer(ctx context.Context, mux *http.ServeMux) (start func() error, stop func(), url string, err error) {
	logger := ctxlog.Logger(ctx)
	redirectURL := "https://" + fv.HTTPSRedirectAddr
	redirectMux := http.NewServeMux()
	redirectMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logger.Info("redirecting to", "url", redirectURL)
		http.Redirect(w, r, redirectURL, http.StatusMovedPermanently)
	})
	redirectServer := &http.Server{
		Addr:              fv.HTTPAddr,
		Handler:           redirectMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	server := &http.Server{
		Addr:              fv.HTTPSAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	start = func() error {
		var g errgroup.T
		g.Go(func() error {
			logger.Info("starting web server", "url", url)
			return server.ListenAndServeTLS(fv.CertFile, fv.KeyFile)
		})
		g.Go(func() error {
			logger.Info("starting redirect server", "url", url, "redirect", fv.HTTPSRedirectAddr)
			return redirectServer.ListenAndServe()
		})
		return g.Wait()
	}
	stop = func() {
		var g errgroup.T
		g.Go(func() error {
			return server.Shutdown(ctx)
		})
		g.Go(func() error {
			return redirectServer.Shutdown(ctx)
		})
		_ = g.Wait()
	}
	url = "https://" + fv.HTTPSAddr
	return
}

func (fv WebUIFlags) createHTTPServer(ctx context.Context, mux *http.ServeMux) (start func() error, stop func(), url string, err error) {
	server := &http.Server{
		Addr:              fv.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	start = func() error {
		ctxlog.Info(ctx, "starting web server", "url", url)
		return server.ListenAndServeTLS(fv.CertFile, fv.KeyFile)
	}
	stop = func() {
		_ = server.Shutdown(ctx)
	}
	url = "http://" + fv.HTTPAddr
	return
}

func (fv WebUIFlags) CreateWebServer(ctx context.Context, mux *http.ServeMux) (func() error, string, error) {
	if fv.HTTPSAddr == "" && fv.HTTPAddr == "" {
		return func() error { return nil }, "", nil
	}

	tls := fv.HTTPSAddr != ""
	if tls && (fv.CertFile == "" || fv.KeyFile == "") {
		return func() error { return nil }, "", fmt.Errorf("ssl-cert and ssl-key flags are required for tls")
	}

	var start func() error
	var stop func()
	var url string
	var err error
	if fv.HTTPSAddr != "" {
		start, stop, url, err = fv.createTLSServer(ctx, mux)
	} else {
		start, stop, url, err = fv.createHTTPServer(ctx, mux)
	}
	if err != nil {
		return func() error { return nil }, "", err
	}

	cmdutil.HandleSignals(func() {
		stop()
	}, os.Interrupt)

	return start, url, nil
}
