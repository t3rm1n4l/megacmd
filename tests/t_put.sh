#!/bin/bash
. environ.bash

init_env
silent dd if=/dev/urandom of=$JUNK/x.1 bs=1k count=50
silent dd if=/dev/urandom of=$JUNK/x.2 bs=1k count=100


run $MEGACMD put $JUNK/x.1 mega:/testing/
expected='mega:/testing/x.1                                  51200'
run $MEGACMD list mega:/testing/
if ! grep -q "$expected" $OUT;
then
    fail Unexpected result
fi

run_fail $MEGACMD put $JUNK/x.1 mega:/testing/
run $MEGACMD -force put $JUNK/x.1 mega:/testing/

expected="mega:/testing/x.1                                  51200"
run $MEGACMD list mega:/testing/
if ! grep -q "$expected" $OUT;
then
    fail Unexpected result
fi

run $MEGACMD -force put $JUNK/x.2 mega:/testing/x.2
expected="mega:/testing/x.2                                  102400"
run $MEGACMD list mega:/testing/
if ! grep -q "$expected" $OUT;
then
    fail Unexpected result
fi

run_fail $MEGACMD -force put $JUNK/x.2 mega:/testing/x.2/

run_fail $MEGACMD put $JUNK/x.1 mega:/testing/non-existent/
run $MEGACMD mkdir  mega:/testing/newdir
run_fail $MEGACMD put $JUNK/x.1 mega:/testing/newdir
run $MEGACMD put $JUNK/x.1 mega:/testing/newdir/
run_fail $MEGACMD put $JUNK/x.1 mega:/testing/newdir/x.1

