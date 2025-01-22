// Copyright 2025 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package webassets

import (
	"embed"
	"fmt"
	"html/template"
)

//go:embed static/*
var Static embed.FS

func readContentsOrDie(path string) []byte {
	b, err := Static.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("failed to read embedded file %v: %v", path, err))
	}
	return b
}

func createTemplateOrDie(path string) *template.Template {
	tpl, err := template.New("testServerIndex").Parse(string(readContentsOrDie(path)))
	if err != nil {
		panic(fmt.Sprintf("failed to create template from %v: %v", path, err))
	}
	return tpl
}
