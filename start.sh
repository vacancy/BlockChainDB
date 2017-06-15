#!/bin/bash
# this script will pass all arguments to server/main.
# Usage: ./start.sh --id=1 -> server/main --id=1
exec server/main "$*"
