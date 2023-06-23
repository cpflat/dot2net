#!/bin/bash

NAME=$1

rm -rf r?/* > /dev/null 2>&1
rmdir r? > /dev/null 2>&1

TOPOLOGY=`wc -c ${NAME}.dot | awk '{print $1}'`
CONFIG=`wc -c ${NAME}.yaml | awk '{print $1}'`
TOTAL=`expr ${TOPOLOGY} + ${CONFIG}`
dot2net tinet -c ${NAME}.yaml ${NAME}.dot > spec.yaml
TINET=`wc -c spec.yaml r?/* host?/* | grep total | awk '{print $1}'`
rm -rf r?/*
rmdir r?
dot2net clab -c ${NAME}.yaml ${NAME}.dot > topo.yaml
CLAB=`wc -c topo.yaml r?/* host?/* | grep total | awk '{print $1}'`
rm -rf r?/*
rmdir r?

echo ${TOPOLOGY} \& \& ${CONFIG} \& \& ${TOTAL} \& \& ${CLAB} \& \& ${TINET}

TOPOLOGY2=`wc -c ${NAME}2.dot | awk '{print $1}'`
CONFIG2=`wc -c ${NAME}.yaml | awk '{print $1}'`
TOTAL2=`expr ${TOPOLOGY2} + ${CONFIG2}`
dot2net tinet -c ${NAME}.yaml ${NAME}2.dot > spec2.yaml
TINET2=`wc -c spec2.yaml r?/* host?/* | grep total | awk '{print $1}'`
rm -rf r?/*
rmdir r?
dot2net clab -c ${NAME}.yaml ${NAME}2.dot > topo2.yaml
CLAB2=`wc -c topo2.yaml r?/* host?/* | grep total | awk '{print $1}'`
rm -rf r?/*
rmdir r?

TOPOLOGY_DIFF=`expr ${TOPOLOGY2} - ${TOPOLOGY}`
TOTAL_DIFF=`expr ${TOTAL2} - ${TOTAL}`
TINET_DIFF=`expr ${TINET2} - ${TINET}`
CLAB_DIFF=`expr ${CLAB2} - ${CLAB}`

echo ${TOPOLOGY2} \& \(\+${TOPOLOGY_DIFF}\) \& ${CONFIG2} \& \(\$\\pm\$0\) \& ${TOTAL2} \& \(+${TOTAL_DIFF}\) \& ${CLAB2} \& \(+${CLAB_DIFF}\) \& ${TINET2} \& \(+${TINET_DIFF}\)

