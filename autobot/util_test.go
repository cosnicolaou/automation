// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"testing"
)

func TestZIP(t *testing.T) {
	builtin, err := loadZIPDatabase("")
	if err != nil {
		t.Fatalf("failed to load ZIP database: %v", err)
	}
	withuk, err := loadZIPDatabase("./testdata")
	if err != nil {
		t.Fatalf("failed to load ZIP database: %v", err)
	}

	for _, tc := range []struct {
		zl   zipLookup
		zip  string
		lat  float64
		long float64
	}{
		{builtin, "CA 95014", 37.318, -122.0449},
		{withuk, "ENG CB4 3EN", 52.2169, 0.1185},
		{withuk, "eng CB4 3EN", 52.2169, 0.1185},
		{withuk, "eng CB4", 52.2228, 0.1305},
	} {

		lat, long, err := tc.zl.Lookup(tc.zip)
		if err != nil {
			t.Errorf("failed to find %q: %v", tc.zip, err)
			continue
		}
		if got, want := lat, tc.lat; got != want {
			t.Errorf("%v: got %v, want %v", tc.zip, got, want)
		}
		if got, want := long, tc.long; got != want {
			t.Errorf("%v: got %v, want %v", tc.zip, got, want)
		}
	}
}
