#! /bin/bash -e

mkdir -p \
  mock

mockgen github.com/nanobox-io/golang-discovery Generator > mock/mock.go
