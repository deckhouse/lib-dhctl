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

package yaml

import (
	"io"
	"regexp"
	"strings"
)

var yamlSplitRegexp = regexp.MustCompile(`(?:^|\s*\n)---\s*`)

func SplitYAML(s string) []string {
	return yamlSplitRegexp.Split(strings.TrimSpace(s), -1)
}

func SplitYAMLBytes(s []byte) []string {
	return SplitYAML(string(s))
}

func SplitYAMLReader(reader io.Reader) ([]string, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return SplitYAML(string(content)), nil
}
