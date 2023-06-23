#!/bin/bash

TIME="/usr/bin/time -v"
NAME=$1

echo "clab"
${TIME} dot2net clab -c ${NAME}.yaml ${NAME}.dot 2> clab.txt > /dev/null
cat clab.txt | grep "wall clock"
cat clab.txt | grep "Maximum resident"
rm clab.txt

echo "tinet"
${TIME} dot2net tinet -c ${NAME}.yaml ${NAME}.dot 2> tinet.txt > /dev/null
cat tinet.txt | grep "wall clock"
cat tinet.txt | grep "Maximum resident"
rm tinet.txt

