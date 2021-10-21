// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integrationtest

import (
	"github.com/google/go-cmp/cmp"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/protobuf/testing/protocmp"
)

// Diff uses cmp.Diff(), protocmp, and some custom options to compare two protobuf messages.
func DiffProtos(x interface{}, y interface{}) string {
	return cmp.Diff(
		x,
		y,
		protocmp.Transform(),

		// Ignore project ID because the fixture may have been dumped from another project.
		protocmp.IgnoreFields(&monitoringpb.CreateTimeSeriesRequest{}, "name"),
		protocmp.IgnoreFields(&monitoringpb.CreateMetricDescriptorRequest{}, "name"),
		protocmp.IgnoreFields(&metricpb.MetricDescriptor{}, "name"),
	)
}
