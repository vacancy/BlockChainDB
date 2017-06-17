#!/bin/bash

./kill.sh

./start.sh -id=2 >term/2.out 2>term/2.err &
echo $! > term/pid
./start.sh -id=3 >term/3.out 2>term/3.err &
echo $! >> term/pid

./start.sh -id=1
