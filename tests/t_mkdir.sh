#!/bin/bash
. environ.bash

init_env

run $MEGACMD mkdir mega:/testing/dir1/dir2
run $MEGACMD mkdir mega:/testing/dir1/abc
run $MEGACMD mkdir mega:/testing/dir1

run $MEGACMD -recursive list mega:/testing/
count=`wc -l $OUT | awk '{ print $1 }'`
if [ $count -ne 3 ];
then
    fail Count mismatch $count
fi


run $MEGACMD mkdir trash:/junk
run $MEGACMD list trash:/
if ! grep -q "junk" $OUT;
then
    fail "Directory not found"
fi
