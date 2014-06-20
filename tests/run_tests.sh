#!/bin/bash
# Description: Tests execution wrapper
# $1 holds the megacmd executable name

trap exit SIGINT

echo Setting up test env
cd `dirname ${BASH_SOURCE[*]}`

./setup.sh

export MEGACMD_NAME=$1

for t in t_*.sh;
do
    echo "#### EXECUTING TEST $t ####"
    ./$t
    if [ $? -ne 0 ];
    then
        exit 1
    fi
    echo "#### COMPLETED TEST $t ###"
    echo
done

