#!/bin/bash

cd /src

echo "Installing dependencies"
go mod download

echo "Running blockbook application"
go run blockbook.go -tags 'rocksdb_6_16' --debug --blockchaincfg=/src/config.json