/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This file contains logic to encapsulate flags which are needed to specify
// what cluster, etc. to use for e2e tests.

package test

import "os"

var (
	// EventingOperatorNamespace is the default namespace for eventing operator e2e tests
	EventingOperatorNamespace = getenv("TEST_NAMESPACE", "operator-tests")
	// EventingOperatorName is the default operator name for eventing operator e2e tests
	EventingOperatorName = getenv("TEST_RESOURCE", "knative-eventing")
)

func getenv(name, defaultValue string) string {
	value, set := os.LookupEnv(name)
	if !set {
		value = defaultValue
	}
	return value
}
