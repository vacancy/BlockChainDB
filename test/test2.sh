#!/bin/bash

./example/test_client -T 5 -from 00000000 -to 12345678 -value 10
./example/test_client -T 5 -from 12345678 -to 00000000 -value 2000
./example/test_client -T 5 -from 00000000 -to 12345678 -value 1000
./example/test_client -T 5 -from 00000001 -to 12345678 -value 1000
