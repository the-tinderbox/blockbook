#!/bin/bash

# Собираем образ для запуска
docker build --platform linux/x86_64 -t blockbook-local-run build/docker/local-run
docker stop blockbook
docker rm blockbook
docker run --name blockbook --platform linux/x86_64 -d -v "$PWD/bin:/src" -v "$PWD/static:/static" -p 9198:9198 -p 9098:9098 blockbook-local-run
docker logs -f blockbook