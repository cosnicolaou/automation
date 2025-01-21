// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"cloudeng.io/cmdutil/keystore"
	"cloudeng.io/geospatial/zipcode"
	"github.com/cosnicolaou/automation/cmd/autobot/internal"
	"github.com/cosnicolaou/automation/cmd/autobot/internal/zipfs"
	"github.com/cosnicolaou/automation/devices"
	"github.com/cosnicolaou/automation/scheduler"
)

func loadSystem(ctx context.Context, fv *ConfigFileFlags, opts ...devices.Option) (context.Context, devices.System, error) {
	keys, err := ReadKeysFile(ctx, fv.KeysFile)
	if err != nil {
		return nil, devices.System{}, fmt.Errorf("failed to read keys file: %q: %w", fv.KeysFile, err)
	}

	zdb, err := loadZIPDatabase(fv.ZIPDatabase)
	if err != nil {
		return nil, devices.System{}, fmt.Errorf("failed to load zip database: %q: %w", fv.ZIPDatabase, err)
	}
	opts = append(opts, devices.WithZIPCodeLookup(zdb))

	system, err := devices.ParseSystemConfigFile(ctx, fv.SystemFile, opts...)
	if err != nil {
		return nil, devices.System{}, fmt.Errorf("failed to parse system config file: %q: %w", fv.SystemFile, err)
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

type zipLookup struct {
	*zipcode.DB
}

func (z zipLookup) Lookup(zip string) (float64, float64, error) {
	zip = strings.ToUpper(zip)
	parts := strings.FieldsFunc(zip, func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	})
	admin, code := "", ""
	switch len(parts) {
	case 2:
		admin, code = parts[0], parts[1]
	case 3:
		admin, code = parts[0], parts[1]+" "+parts[2]
	default:
		return 0, 0, fmt.Errorf("invalid zipcode: %v", zip)
	}
	if ll, ok := z.LatLong(admin, code); ok {
		return ll.Lat, ll.Long, nil
	}
	return 0, 0, fmt.Errorf("unknown zipcode: %v", zip)
}

func loadZIPDatabaseDir(db *zipcode.DB, lfs fs.FS) error {
	return fs.WalkDir(lfs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".zip" {
			fmt.Printf("loading zip file: %v\n", path)
			if err := internal.LoadFromZIPArchive(db, lfs, path); err != nil {
				return fmt.Errorf("failed to load database file: %v, %v", path, err)
			}
			return nil
		}
		fmt.Printf("loading zip file: %v\n", path)
		data, err := fs.ReadFile(lfs, path)
		if err != nil {
			return err
		}
		if err := db.Load(data); err != nil {
			return fmt.Errorf("failed to load database file: %v, %v", path, err)
		}
		return nil
	})
}

func loadZIPDatabase(dbdir string) (zipLookup, error) {
	db := zipcode.NewDB()
	if len(dbdir) == 0 {
		var lfs fs.FS = zipfs.USZipCodes
		if err := internal.LoadFromZIPArchive(db, lfs, "US.zip"); err != nil {
			return zipLookup{}, fmt.Errorf("failed to load embedded US zipcode database: %v", err)
		}
		return zipLookup{DB: db}, nil
	}
	lfs := os.DirFS(dbdir)
	if err := loadZIPDatabaseDir(db, lfs); err != nil {
		return zipLookup{}, fmt.Errorf("failed to load zipcode database from directory %v: %v", dbdir, err)
	}
	return zipLookup{DB: db}, nil
}
