// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"cloudeng.io/cmdutil/keystore"
	"cloudeng.io/geospatial/zipcode"
	"github.com/cosnicolaou/automation/autobot/internal"
	"github.com/cosnicolaou/automation/devices"
	"github.com/cosnicolaou/automation/scheduler"
)

func loadSystem(ctx context.Context, fv *ConfigFileFlags, opts ...devices.Option) (context.Context, devices.System, error) {
	keys, err := ReadKeysFile(ctx, fv.KeysFile)
	if err != nil {
		return nil, devices.System{}, err
	}

	zdb, err := loadZIPDatabase(ctx, fv.ZIPDatabase)
	if err != nil {
		return nil, devices.System{}, err
	}
	opts = append(opts, devices.WithZIPCodeLookup(zdb))

	system, err := devices.ParseSystemConfigFile(ctx, fv.SystemFile, opts...)
	if err != nil {
		return nil, devices.System{}, err
	}
	return keystore.ContextWithAuth(ctx, keys), system, nil
}

func loadSchedules(ctx context.Context, fv *ConfigFileFlags, sys devices.System) (scheduler.Schedules, error) {
	if fv.ScheduleFile == "" {
		return scheduler.Schedules{}, fmt.Errorf("no schedule file specified")
	}
	cfg, err := os.ReadFile(fv.ScheduleFile)
	if err != nil {
		return scheduler.Schedules{}, fmt.Errorf("failed to read schedule file: %q: %v", fv.ScheduleFile, err)
	}
	scheds, err := scheduler.ParseConfig(ctx, cfg, sys)
	if err != nil {
		return scheduler.Schedules{}, fmt.Errorf("failed to parse schedule file: %q: %v", fv.ScheduleFile, err)
	}
	return scheds, nil
}

//go:embed US.zip
var USZipCodes embed.FS

type zipLookup struct {
	*zipcode.DB
}

func (z zipLookup) Lookup(zip string) (float64, float64, error) {
	parts := strings.FieldsFunc(zip, func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	})
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid zipcode: %v", zip)
	}
	if ll, ok := z.LatLong(parts[0], parts[1]); ok {
		return ll.Lat, ll.Long, nil
	}
	return 0, 0, fmt.Errorf("unknown zipcode: %v", zip)
}

func loadZIPDatabase(ctx context.Context, dbname string) (zipLookup, error) {
	filename := "US.zip"
	var lfs fs.FS = USZipCodes
	if dbname != "" {
		dirname := filepath.Dir(dbname)
		filename = filepath.Base(dbname)
		lfs = os.DirFS(dirname)
	}
	db := zipcode.NewDB()
	if err := internal.LoadFromZIPArchive(db, lfs, filename); err != nil {
		return zipLookup{}, fmt.Errorf("failed to load embedded US zipcode database: %v\n", err)
	}
	return zipLookup{DB: db}, nil
}
