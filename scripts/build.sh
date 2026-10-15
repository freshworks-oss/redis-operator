#!/usr/bin/env bash

set -o errexit
set -o nounset

src=./cmd/redisoperator
out=./bin/redis-operator

# Set default values if not defined
export GOOS=${TARGETOS:-$(go env GOOS)}
export GOARCH=${TARGETARCH:-$(go env GOARCH)}

echo "Building ${GOOS}/${GOARCH} binary at ${out}"
CGO_ENABLED=0 go build -trimpath -o "${out}" -ldflags "-s -w" "${src}"