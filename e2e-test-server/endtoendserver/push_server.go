// Copyright 2023 Google LLC
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

package endtoendserver

import (
	"context"
)

// pushServer is an end-to-end test service.
type pushServer struct{}

// New instantiates a new push end-to-end test service.
func NewPushServer() (*pushServer, error) {
	return &pushServer{}, nil
}

// Run the end-to-end test service. This method will block until the context is
// cancel, or an unrecoverable error is encountered.
func (s *pushServer) Run(ctx context.Context) error {
	return nil
}

// Shutdown gracefully shuts down the service, flushing and closing resources as
// appropriate.
func (s *pushServer) Shutdown(ctx context.Context) {}
