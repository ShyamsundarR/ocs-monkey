#!/bin/bash

# A very basic build script

CONTAINER_REPO="quay.io/shyamsundarr/filecounts:test"
BUILDDATE=$(date -u '+%Y-%m-%dT%H:%M:%S.%NZ')
VERSION="0.1"

go build -o filecounts

docker build \
        -t "${CONTAINER_REPO}" \
        --build-arg "builddate=$BUILDDATE" \
        --build-arg "version=$VERSION" \
        .
