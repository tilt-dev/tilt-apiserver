#!/usr/bin/env bash

set -e

DIR=$(dirname "$0")
cd "$DIR/.."

goversion=$(grep "^go " go.mod | sed 's/go //')

exec docker run -e "CODEGEN_UID=$(id -u)" -e "CODEGEN_GID=$(id -g)" -v "$(pwd)":/go/src/github.com/tilt-dev/tilt-apiserver --workdir /go/src/github.com/tilt-dev/tilt-apiserver \
     --entrypoint ./hack/update-codegen-helper.sh \
   golang:$goversion
