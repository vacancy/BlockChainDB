#!/bin/bash

for I in `seq 0 9`; do
	./example/test_client -T=TRANSFER --from USER000$I --to USER0099 --value=10 --fee=5
done

#./example/test_client -T 5 -from 00000000 -to 12345678 -value 10
#./example/test_client -T 5 -from 12345678 -to 00000000 -value 2000
#./example/test_client -T 5 -from 00000000 -to 12345678 -value 1000
#./example/test_client -T 5 -from 00000001 -to 12345678 -value 1000
