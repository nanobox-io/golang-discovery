#! /bin/bash -e

mkdir -p \
  mock \
  mock-closer

mockgen github.com/nanobox-io/golang-discovery Generator > mock/mock.go
mockgen io Closer > mock-closer/io-closer.go
