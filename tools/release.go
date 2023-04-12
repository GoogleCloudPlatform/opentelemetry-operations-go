// Copyright 2021 Google LLC
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

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	prefix = "github.com/GoogleCloudPlatform/opentelemetry-operations-go"

	stable   = "1.13.1"
	unstable = "0.37.1"
)

var versions = map[string]string{
	"":                    unstable,
	"exporter/trace/":     stable,
	"example/trace/http/": unstable,

	"exporter/metric/": unstable,
	"example/metric/":  unstable,

	"exporter/collector/":                         unstable,
	"exporter/collector/googlemanagedprometheus/": unstable,

	"internal/resourcemapping/": unstable,
	"internal/cloudmock/":       unstable,

	"detectors/gcp/": stable,

	"propagator/": unstable,

	"e2e-test-server/": unstable,
}

type module string

func allModules() ([]module, error) {
	var out []module
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == "go.mod" {
			out = append(out, module(path))
		}

		return nil
	})
	return out, err
}

func (m module) requires() ([]string, error) {
	cmd := exec.Command("go", "mod", "edit", "-json", string(m))
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running %q: %w", cmd.String(), err)
	}
	var parsed struct {
		Require []struct{ Path string }
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("parsing output from %q: %w", cmd.String(), err)
	}
	result := make([]string, len(parsed.Require))
	for i, req := range parsed.Require {
		result[i] = req.Path
	}
	return result, nil
}

func (m module) editRequirement(dep, version string) error {
	cmd := exec.Command("go", "mod", "edit", "-require="+dep+"@v"+version, string(m))
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running %q: %w", cmd.String(), err)
	}
	return nil
}

func (m module) version() string {
	modDir := filepath.Dir(string(m))
	if modDir == "." {
		modDir = ""
	} else {
		modDir += "/"
	}
	ver := versions[modDir]
	return ver
}

var pattern = regexp.MustCompile(`return ".*"`)

func (m module) updateVersion() error {
	fullPath := filepath.Join(filepath.Dir(string(m)), "version.go")
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return err
	}

	content = pattern.ReplaceAllLiteral(content, []byte(`return "`+m.version()+`"`))
	return os.WriteFile(fullPath, content, 0)
}

func prepare() error {
	mods, err := allModules()
	if err != nil {
		return err
	}
	for _, m := range mods {
		if err := m.updateVersion(); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}

		deps, err := m.requires()
		if err != nil {
			return err
		}

		for _, dep := range deps {
			if !strings.HasPrefix(dep, prefix) {
				continue
			}
			suffix := strings.TrimPrefix(dep, prefix)
			if suffix != "" {
				suffix = suffix[1:] + "/"
			}
			ver := versions[suffix]
			if err := m.editRequirement(dep, ver); err != nil {
				return err
			}
		}
	}

	return nil
}

func tag() error {
	for dir, ver := range versions {
		tag := dir + "v" + ver
		fmt.Printf("Creating tag %s\n", tag)
		cmd := exec.Command("git", "tag", tag)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("running %q: %w", cmd.String(), err)
		}
	}
	return nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run tools/release.go (prepare|tag)")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "prepare":
		if err := prepare(); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Changes needed for release are in the working tree. Commit them and make a PR for review.")

	case "tag":
		if err := tag(); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Now push the newly created tags to Github using git push --tags")
	}

}
