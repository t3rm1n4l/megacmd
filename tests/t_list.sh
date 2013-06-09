#!/bin/bash
. environ.bash

init_env

echo "123 5678" > $JUNK/random.txt
run $MEGACMD mkdir mega:/testing/dir1-1/dir2/dir3
run $MEGACMD mkdir mega:/testing/dir1-2/dir2/dir3

run $MEGACMD list mega:/testing/
count=`wc -l $OUT | awk '{ print $1 }'`
if [ $count -ne 2 ];
then
    fail Count mismatch $count
fi

run $MEGACMD put $JUNK/random.txt mega:/testing/dir1-2/dir2/dir3/

run $MEGACMD list mega:/testing/
count=`wc -l $OUT | awk '{ print $1 }'`
if [ $count -ne 2 ];
then
    fail Count mismatch $count
fi

run $MEGACMD -recursive list mega:/testing/
count=`wc -l $OUT | awk '{ print $1 }'`
if [ $count -ne 7 ];
then
    fail Count mismatch $count
fi

run $MEGACMD -recursive list mega:/testing/
count=`wc -l $OUT | awk '{ print $1 }'`
if [ $count -ne 7 ];
then
    fail Count mismatch $count
fi

expected="mega:/testing/dir1-2/dir2/dir3/random.txt          9"
run $MEGACMD list mega:/testing/dir1-2/dir2/dir3/
if [[ ! `cat $OUT` =~ "$expected" ]];
then
    fail Unexpected result
fi

run $MEGACMD list mega:/testing/dir1-2/dir2/dir3/random.txt
if [[ ! `cat $OUT` =~ "$expected" ]];
then
    fail Unexpected result
fi
