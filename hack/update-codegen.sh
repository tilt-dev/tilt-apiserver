#!/usr/bin/env bash

set -e

DIR=$(dirname "$0")
cd "$DIR/.."

exec docker run -v "$(pwd)":/go/src/github.com/tilt-dev/tilt-apiserver --workdir /go/src/github.com/tilt-dev/tilt-apiserver \
     --entrypoint ./hack/update-codegen-helper.sh \
   golang:1.17
