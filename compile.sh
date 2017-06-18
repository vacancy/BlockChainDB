#!/bin/bash

cd server
go build main.go `ls *.go | grep -v main`

exit $!
