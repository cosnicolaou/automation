// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package internal

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"

	"cloudeng.io/geospatial/zipcode"
)

func LoadFromZIPArchive(zdb *zipcode.DB, fsys fs.FS, filename string) error {
	data, err := fs.ReadFile(fsys, filename)
	if err != nil {
		return err
	}
	zar, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("failed to read zip archive: %v %v", filename, err)
	}
	for _, file := range zar.File {
		if file.Name == "readme.txt" {
			continue
		}
		f, err := zar.Open(file.Name)
		if err != nil {
			return fmt.Errorf("failed to open file: %v in archive %v: %v", file.Name, filename, err)
		}
		data, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf("failed to read file: %v in archive %v: %v", file.Name, filename, err)
		}
		if err := zdb.Load(data); err != nil {
			return fmt.Errorf("failed to load data from file: %v in archive %v: %v", file.Name, filename, err)
		}
	}
	return nil
}

/*
func (zdb *DB) LoadFile(fsys fs.FS, filename string) error {
	if strings.HasSuffix(filename, ".zip") {
		return zdb.LoadFromZIPArchive(fsys, filename)
	}
	data, err := fs.ReadFile(fsys, filename)
	if err != nil {
		return err
	}
	return zdb.LoadData(data)
}

func (zdb *DB) LoadFromZIPArchive(fsys fs.FS, filename string, opts ...OIpt) error {
	data, err := fs.ReadFile(fsys, filename)
	if err != nil {
		return err
	}
	zar, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("failed to read zip archive: %v %v", filename, err)
	}
	for _, file := range zar.File {
		if file.Name == "readme.txt" {
			continue
		}
		f, err := zar.Open(file.Name)
		if err != nil {
			return fmt.Errorf("failed to open file: %v in archive %v: %v", file.Name, filename, err)
		}
		data, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf("failed to read file: %v in archive %v: %v", file.Name, filename, err)
		}
		if err := zdb.LoadData(data); err != nil {
			return fmt.Errorf("failed to load data from file: %v in archive %v: %v", file.Name, filename, err)
		}
	}
	return nil
}

*/
/*

	scanner := bufio.NewScanner(file)
	zipcodeMap := Zipcodes{DatasetList: make(map[string]ZipCodeLocation)}
	for scanner.Scan() {
		splittedLine := strings.Split(scanner.Text(), "\t")
		if len(splittedLine) != 12 {
			return Zipcodes{}, fmt.Errorf("zipcodes: file line does not have 12 fields")
		}
		lat, errLat := strconv.ParseFloat(splittedLine[9], 64)
		if errLat != nil {
			return Zipcodes{}, fmt.Errorf("zipcodes: error while converting %s to Latitude", splittedLine[9])
		}
		lon, errLon := strconv.ParseFloat(splittedLine[10], 64)
		if errLon != nil {
			return Zipcodes{}, fmt.Errorf("zipcodes: error while converting %s to Longitude", splittedLine[10])
		}

		zipcodeMap.DatasetList[splittedLine[1]] = ZipCodeLocation{
			ZipCode:   splittedLine[1],
			PlaceName: splittedLine[2],
			AdminName: splittedLine[3],
			State:     splittedLine[4],
			Lat:       lat,
			Lon:       lon,
		}
	}
*/
