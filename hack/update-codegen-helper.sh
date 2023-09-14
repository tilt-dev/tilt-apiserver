#!/usr/bin/env bash

# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

GOPATH=$(go env GOPATH)
export GOPATH
SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

git config --global --add safe.directory /go/src/github.com/tilt-dev/tilt-apiserver

# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.
source "${CODEGEN_PKG}/kube_codegen.sh"

kube::codegen::gen_helpers \
  --input-pkg-root github.com/tilt-dev/tilt-apiserver/pkg/apis \
  --output-base "$(dirname "${BASH_SOURCE[0]}")/../../../.." \
  --boilerplate "${SCRIPT_ROOT}"/hack/boilerplate.go.txt

kube::codegen::gen_client \
  --with-watch \
  --with-applyconfig \
  --input-pkg-root github.com/tilt-dev/tilt-apiserver/pkg/apis \
  --output-pkg-root github.com/tilt-dev/tilt-apiserver/pkg/generated \
  --output-base "$(dirname "${BASH_SOURCE[0]}")/../../../.." \
  --boilerplate "${SCRIPT_ROOT}"/hack/boilerplate.go.txt

