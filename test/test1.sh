#!/bin/bash

./example/test_client -T 5 -from 00000000 -to 12345678 -value 10
./example/test_client -T 5 -from 00000000 -to 12345678 -value 8
./example/test_client -T 5 -from 00000000 -to 12345678 -value 6
./example/test_client -T 5 -from 00000000 -to 12345678 -value 4
./example/test_client -T 5 -from 00000000 -to 12345678 -value 2 -fee 2

sleep 10

./example/test_client -T 1 -user 00000000
echo "expected value for user 00000000: 972"
./example/test_client -T 1 -user 12345678
echo "expected value for user 12345678: 1024"
