#!/bin/bash

# Удаляем старые данные
rm ./bin/blockbook
#rm -rf ./bin/data
rm -rf ./bin/logs/*

# Собираем образ
docker build --platform linux/x86_64 -t blockbook-local build/docker/local-bin
docker run --platform linux/x86_64 -t --rm -e PACKAGER="501:20" -v "$PWD:/src" -v "$PWD/bin:/out" blockbook-local make build

./run-docker.sh