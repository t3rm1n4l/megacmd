#!/bin/bash

echo Setting up test env
cd `dirname ${BASH_SOURCE[*]}`

./setup.sh

for t in t_*.sh;
do
    echo "#### EXECUTING TEST $t ####"
    ./$t
    if [ $exit_status -eq 0 ];
    then
        exit_status=$?
    fi
    echo "#### COMPLETED TEST $t ###"
    echo
done

exit exit_status
