// Copyright 2025 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package webassets

import (
	"embed"
)

//go:embed static/*
var Static embed.FS

/*
func readContents(cfs fs.FS, path string) ([]byte, error) {
	f, err := cfs.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func createTemplateOrDie(fs fs.FS, path string) (*template.Template, error) {
	c, err := readContents(fs, path)
	if err != nil {
		return nil, err
	}
	tpl, err := template.New(path).Parse(string(c))
	if err != nil {
		panic(fmt.Sprintf("failed to create template from %v: %v", path, err))
	}
	return tpl, nil
}
*/
