// Copyright 2025 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package webassets

import (
	"embed"
)

//go:embed static/*
var Static embed.FS
