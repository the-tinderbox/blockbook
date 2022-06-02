#!/bin/bash

# Собираем образ для запуска
echo "Building base docker image"
docker build -t blockbook-local-base -f build/docker/local-go-run/Dockerfile-base .

echo "Building runner docker image"
docker build -t blockbook-local-runner -f build/docker/local-go-run/Dockerfile-runner .

echo "Trying to stop container"
docker stop blockbook

docker start blockbook

echo "Removing old container"
#docker rm blockbook

echo "Running new container"
docker run --name blockbook -d -v "$PWD:/code" -v "$PWD/static:/static" -v "$PWD/bin:/blockbook" -p 9198:9198 -p 9098:9098 blockbook-local-runner
#docker logs -f blockbook