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

package endtoendserver

import (
	"context"
	"fmt"
)

// Server is an end-to-end test service.
type Server interface {
	Run(context.Context) error
	Shutdown(ctx context.Context) error
}

// New instantiates a new end-to-end test service.
func New() (Server, error) {
	switch subscriptionMode {
	case "pull":
		return NewPullServer()
	case "push":
		return NewPushServer()
	default:
		return nil, fmt.Errorf("server does not support subscription mode %v", subscriptionMode)
	}
}
