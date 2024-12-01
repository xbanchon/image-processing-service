#!/bin/bash

apt install libvips libvips-dev -y
go build -C cmd/app -o out
