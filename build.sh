#!/bin/bash

date="$(date -u +'%Y-%m-%dT%TZ%z')"
commit=$(git rev-parse HEAD)
version="$(git symbolic-ref -q --short HEAD || git describe --tags --exact-match)"
go build -ldflags "-X main.commit=$commit -X main.buildDate=$date -X main.version=$version" -o dist/patroneosd