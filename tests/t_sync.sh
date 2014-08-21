#!/bin/bash
. environ.bash

init_env

mkdir -p $JUNK/sync1/dirx/diry/dirz
mkdir -p $JUNK/sync1/dira
mkdir -p $JUNK/sync1/dirb/dirc/dird

silent dd if=/dev/urandom of=$JUNK/sync1/dira/x.1 bs=1k count=1
silent dd if=/dev/urandom of=$JUNK/sync1/x.2 bs=1k count=1
silent dd if=/dev/urandom of=$JUNK/sync1/dirb/dirc/dird/x.3 bs=1k count=1

run_fail $MEGACMD sync nothing mega:/testing/sync

run $MEGACMD sync $JUNK/sync1 mega:/testing/sync1
run $MEGACMD -recursive list mega:/testing/sync1/
count=`wc -l $OUT | awk '{ print $1 }'`
if [ $count -ne 10 ];
then
    fail Count mismatch $count
fi

run $MEGACMD sync $JUNK/sync1 mega:/testing/newone/sync1
run $MEGACMD -recursive list mega:/testing/newone/
count=`wc -l $OUT | awk '{ print $1 }'`
if [ $count -ne 11 ];
then
    fail Count mismatch $count
fi

run_fail $MEGACMD sync $JUNK/sync1 mega:/testing/sync1
run $MEGACMD -force sync $JUNK/sync1 mega:/testing/sync1

run $MEGACMD sync mega:/testing/sync1 $JUNK/sync2
count=`find $JUNK/sync2 | wc -l | awk '{ print $1 }'`
if [ $count -ne 11 ];
then
    fail Count mismatch $count
fi

run_fail $MEGACMD sync mega:/testing/syncn $JUNK/sync2
run $MEGACMD sync mega:/testing/sync1 $JUNK/newone




