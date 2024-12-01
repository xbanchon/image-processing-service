#!/bin/bash

sudo apt update && sudo apt install libvips libvips-dev -y
go build -C cmd/app -o out
