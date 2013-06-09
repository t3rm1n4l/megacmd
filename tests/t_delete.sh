#!/bin/bash
. environ.bash

init_env

echo "123 5678" > $JUNK/random.txt
run $MEGACMD mkdir mega:/testing/dir1/dir2
run $MEGACMD mkdir mega:/testing/dir1/abc
run $MEGACMD mkdir mega:/testing/dir1
run $MEGACMD mkdir mega:/testing/dir1/1/2/3/4/5

run $MEGACMD put $JUNK/random.txt mega:/testing/dir1/1/2/new.txt
run $MEGACMD put $JUNK/random.txt mega:/testing/dir1/1/2/new_hard.txt

run $MEGACMD delete mega:/testing/dir1/1/2/new.txt

run $MEGACMD list trash:/
if ! grep -q "new.txt" $OUT;
then
    fail "Soft deleted file not found"
fi

run $MEGACMD -force delete mega:/testing/dir1/1/2/new_hard.txt

run $MEGACMD list trash:/
if grep -q "new_hard.txt" $OUT;
then
    fail "Force deleted file should not appear in trash"
fi

run $MEGACMD -recursive list mega:/
if grep -q "new_hard.txt" $OUT;
then
    fail "Force deleted file should not appear in trash"
fi

run $MEGACMD delete mega:/testing/dir1/1/2
run $MEGACMD -recursive list mega:/dir1/1/
if grep -q -e "2" -e "3" $OUT;
then
    fail "Deleted directory should not appear"
fi

run_fail $MEGACMD delete mega:/testing/dir1/1/2/hello

