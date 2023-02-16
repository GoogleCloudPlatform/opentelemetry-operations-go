// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logsutil

// ExporterConfig configures the Logs exporter with various settings post-initializition.
// It is meant to be used by integration tests.
type ExporterConfig struct {
	// MaxEntrySize is the maximum size of an individual LogEntry in bytes. Entries
	// larger than this size will be split into multiple entries.
	MaxEntrySize int
	// MaxRequestSize is the maximum size of a batch WriteLogEntries request in bytes.
	// Request larger than this size will be split into multiple requests.
	MaxRequestSize int
}
