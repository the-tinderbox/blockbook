#!/bin/bash

go run blockbook.go -debug -blockchaincfg=/blockbook/config.json -sync -internal=:9098 -public=:9198 -log_dir=/blockbook/logs -datadir=/blockbook/data -notxcache