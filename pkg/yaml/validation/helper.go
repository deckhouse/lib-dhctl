// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validation

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-openapi/spec"
)

const defaultSchemasDir = "/deckhouse/candi/openapi"

func loadSchemasDir(schemasDir []string) (map[SchemaIndex]*spec.Schema, error) {
	initMap := make(map[SchemaIndex]*spec.Schema)

	walkFunc := func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return nil
		}

		if err != nil {
			return err
		}

		if strings.HasPrefix(info.Name(), "doc-ru") {
			return nil
		}

		if isYAMLFile(path) {
			content, openError := os.ReadFile(path)
			if openError != nil {
				return openError
			}

			r := bytes.NewReader(content)
			schemas, err := LoadSchemas(r)
			if err == nil {
				for _, schema := range schemas {
					initMap[schema.Index] = schema.Schema
				}
			}
		}

		return nil
	}

	for _, d := range schemasDir {
		err := filepath.Walk(d, walkFunc)
		if err != nil {
			return nil, err
		}
	}

	return initMap, nil
}

func isYAMLFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yml" || ext == ".yaml"
}

func ValidateData(schemasDir []string, data *[]byte) error {
	// add default candi dir
	schemasDir = append(schemasDir, defaultSchemasDir)
	initMap, err := loadSchemasDir(schemasDir)
	if err != nil {
		return fmt.Errorf("failed to load schemas: %w", err)
	}

	validator := NewValidator(initMap)
	_, err = validator.Validate(data)

	return err
}
