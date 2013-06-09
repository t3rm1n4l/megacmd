#!/bin/bash
. environ.bash

init_env

echo "123 5678" > $JUNK/random.txt
run $MEGACMD mkdir mega:/testing/dir1/dir2
run $MEGACMD mkdir mega:/testing/abc/something
run $MEGACMD mkdir mega:/testing/dir1
run $MEGACMD mkdir mega:/testing/dir1/1/2/3/4/5

run $MEGACMD put $JUNK/random.txt mega:/testing/dir1/1/2/new.txt
run $MEGACMD put $JUNK/random.txt mega:/testing/dir1/1/2/new_hard.txt

run $MEGACMD move mega:/testing/dir1 mega:/testing/dir-renamed
run $MEGACMD list mega:/testing/

if ! grep -q "renamed" $OUT
then
    fail "Move did not happen"
fi


run_fail $MEGACMD move mega:/testing/dir-renamed mega:/testing/abc/something
run $MEGACMD move mega:/testing/dir-renamed mega:/testing/abc/something/

run_fail $MEGACMD move mega:/testing/abc/something/dir-renamed/1/2/new.txt mega:/testing/abc/something/dir-renamed/1/2/new_hard.txt
run $MEGACMD move mega:/testing/abc/something/dir-renamed/1/2/new.txt mega:/testing/


run_fail $MEGACMD move mega:/testing/dir-renamed junk

