#!/bin/bash

cd server
go build main.go `find . -name "*.go" | grep -v main`

exit $!
