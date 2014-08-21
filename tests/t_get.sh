#!/bin/bash
. environ.bash

init_env
silent dd if=/dev/urandom of=$JUNK/x.1 bs=1k count=50
silent dd if=/dev/urandom of=$JUNK/x.2 bs=1k count=100
silent dd if=/dev/urandom of=$JUNK/x.3 bs=1k count=500
silent dd if=/dev/urandom of=$JUNK/x.4 bs=1k count=1000

run_fail $MEGACMD get mega:/testing/junkx.1 $JUNK/tmp/
run_fail $MEGACMD get mega:/testing/ $JUNK/tmp/
run_fail $MEGACMD get mega:/testing $JUNK/tmp/


for i in {1..4}
do
    run $MEGACMD put $JUNK/x.$i mega:/testing/
    run $MEGACMD get mega:/testing/x.$i $JUNK/tmp/
    if [ ! -e $JUNK/tmp/x.$i ];
    then
        fail "Downloaded file not found"
    fi

    count=`shasum $JUNK/x.$i $JUNK/tmp/x.$i  | cut -d' ' -f1 | sort -u | wc -l | awk '{ print $1 }'`
    if [ $count -ne 1 ];
    then
        fail "Sha1sum mismatch"
    fi
done


run_fail $MEGACMD get mega:/testing/x.1 $JUNK/tmp/
run_fail $MEGACMD get mega:/testing/x.1 $JUNK/tmp
run $MEGACMD -force get mega:/testing/x.1 $JUNK/tmp/

run $MEGACMD -force get mega:/testing/x.1
