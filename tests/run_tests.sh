#!/bin/bash
# Description: Tests execution wrapper

trap exit SIGINT

echo Setting up test env
cd `dirname ${BASH_SOURCE[*]}`

./setup.sh

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

