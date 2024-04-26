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
set -x

GOPATH=$(go env GOPATH)
export GOPATH
SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

git config --global --add safe.directory /go/src/github.com/tilt-dev/tilt-apiserver

source "${CODEGEN_PKG}/kube_codegen.sh"

rm -fR pkg/apis/**/zz_generated*
kube::codegen::gen_helpers \
  --boilerplate "${SCRIPT_ROOT}"/hack/boilerplate.go.txt \
  ./pkg/apis

rm -fR pkg/generated
mkdir -p pkg/generated
kube::codegen::gen_client \
  --with-watch \
  --with-applyconfig \
  --output-pkg github.com/tilt-dev/tilt-apiserver/pkg/generated \
  --output-dir ./pkg/generated \
  --boilerplate "${SCRIPT_ROOT}"/hack/boilerplate.go.txt \
  ./pkg/apis

mkdir -p pkg/generated/openapi
kube::codegen::gen_openapi \
  --output-pkg github.com/tilt-dev/tilt-apiserver/pkg/generated/openapi \
  --output-dir ./pkg/generated/openapi \
  --report-filename "${SCRIPT_ROOT}"/hack/api_violations.list \
  --update-report \
  --boilerplate "${SCRIPT_ROOT}"/hack/boilerplate.go.txt \
  ./pkg/apis

if [[ "$CODEGEN_UID" != "$(id -u)" ]]; then
    groupadd --gid "$CODEGEN_GID" codegen-user
    useradd --uid "$CODEGEN_UID" -g codegen-user codegen-user

    find pkg | while read f; do
        if [ -d "$f" ]; then
            chmod 775 "$f"
        else
            chmod 664 "$f"
        fi
        chown codegen-user:codegen-user "$f"
    done
fi
