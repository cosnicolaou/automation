// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"

	"cloudeng.io/cmdutil/keys"
	"cloudeng.io/cmdutil/keys/unsafekeystore"
)

func ReadKeysFile(ctx context.Context, path string) (context.Context, error) {
	ims := keys.NewInMemoryKeyStore()
	fs := unsafekeystore.New()
	if err := ims.ReadYAML(ctx, fs, path); err != nil {
		return nil, err
	}
	return keys.ContextWithKeyStore(ctx, ims), nil
}
