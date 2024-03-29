/*


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

package main

import (
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder"
	"k8s.io/klog/v2"

	// +kubebuilder:scaffold:resource-imports
	corev1alpha1 "github.com/tilt-dev/tilt-apiserver/pkg/apis/core/v1alpha1"
	tiltopenapi "github.com/tilt-dev/tilt-apiserver/pkg/generated/openapi"
)

func main() {
	builder := builder.NewServerBuilder().
		WithResourceMemoryStorage(&corev1alpha1.Manifest{}, "data").
		WithOpenAPIDefinitions("tilt", "0.1.0", tiltopenapi.GetOpenAPIDefinitions)

	err := builder.ExecuteCommand()
	if err != nil {
		klog.Fatal(err)
	}
}
