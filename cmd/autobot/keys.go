// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"

	"cloudeng.io/cmdutil/keystore"
)

func ReadKeysFile(ctx context.Context, path string) (keystore.Keys, error) {
	return keystore.ParseConfigURI(ctx, path, URIHandlers)
}
