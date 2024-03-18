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

package googleclientauthextension // import "github.com/GoogleCloudPlatform/opentelemetry-operations-go/extension/googleclientauthextension"

import (
	"context"
	"errors"

	"google.golang.org/grpc/credentials"
)

// perRPCCredentials returns gRPC credentials using the OAuth TokenSource, and adds
// google metadata.
func (ca clientAuthenticator) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	if ca.TokenSource == nil {
		return nil, errors.New("not started")
	}
	return ca, nil
}

// GetRequestMetadata gets the request metadata as a map from a clientAuthenticator.
// Based on metadata added by the google go client:
// https://github.com/googleapis/google-api-go-client/blob/113082d14d54f188d1b6c34c652e416592fc51b5/transport/grpc/dial.go#L224
func (ca clientAuthenticator) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	if ca.TokenSource == nil {
		return nil, errors.New("not started")
	}

	metadata, err := ca.TokenSource.GetRequestMetadata(ctx, uri...)
	if err != nil {
		return nil, err
	}

	// Attach system parameters
	if ca.config.QuotaProject != "" {
		metadata["X-goog-user-project"] = ca.config.QuotaProject
	}
	if ca.config.Project != "" {
		metadata["X-goog-project-id"] = ca.config.Project
	}
	return metadata, nil
}
