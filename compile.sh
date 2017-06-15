#!/bin/bash

cd server
go build main.go config.go server.go hash.go crcio.go
