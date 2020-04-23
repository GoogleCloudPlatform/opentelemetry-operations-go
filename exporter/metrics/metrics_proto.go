// Copyright 2018, OpenCensus Authors
//           2020, Google Cloud Operations Exporter Authors
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

package metrics

import (
	"errors"
	"path"
	"strings"
)

var domains = []string{"googleapis.com", "kubernetes.io", "istio.io", "knative.dev"}

var errNilMetricOrMetricDescriptor = errors.New("non-nil metric or metric descriptor")

func (me *metricsExporter) metricTypeFromProto(name string) string {
	prefix := me.o.MetricPrefix
	if me.o.GetMetricPrefix != nil {
		prefix = me.o.GetMetricPrefix(name)
	}
	if prefix != "" {
		name = path.Join(prefix, name)
	}
	if !hasDomain(name) {
		// Still needed because the name may or may not have a "/" at the beginning.
		name = path.Join(defaultDomain, name)
	}
	return name
}

// hasDomain checks if the metric name already has a domain in it.
func hasDomain(name string) bool {
	for _, domain := range domains {
		if strings.Contains(name, domain) {
			return true
		}
	}
	return false
}
