// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package helpers

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	log "github.com/sirupsen/logrus"
)

// FileSaver represents the object that save string or byte data to file
type FileSaver struct{}

// SaveFileString saves string to file
func (f *FileSaver) SaveFileString(dir string, file string, data string) error {
	return f.SaveFile(dir, file, []byte(data))
}

// SaveFile saves binary data to file
func (f *FileSaver) SaveFile(dir string, file string, data []byte) error {
	// don't blindly create directory
	if dir != "" {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if e := os.MkdirAll(dir, 0700); e != nil {
				return fmt.Errorf("error creating directory '%s': %s", dir, e.Error())
			}
		}
	}

	path := path.Join(dir, file)
	if err := ioutil.WriteFile(path, data, 0600); err != nil {
		return err
	}

	log.Debugf("output: wrote %s", path)

	return nil
}
