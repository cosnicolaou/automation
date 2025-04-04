// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package main

import (
	"cloudeng.io/cmdutil/cmdyaml"
	"cloudeng.io/macos/keychainfs"
)

var URIHandlers = map[string]cmdyaml.URLHandler{
	"keychain": keychainfs.NewSecureNoteFSFromURL,
}
