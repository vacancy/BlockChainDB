#!/bin/sh
protoc -I ./ ./db.proto --go_out=plugins=grpc:./go 
