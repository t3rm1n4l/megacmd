#/bin/bash

# Setup environment
export MEGACMD="../$MEGACMD_NAME -conf=t.json -verbose=0"

JUNK="junk"
OUT="$JUNK/out.txt"


function init_env() {
    rm -rf $JUNK
    mkdir -p $JUNK/tmp
    $MEGACMD delete mega:/testing > /dev/null 2>&1
    $MEGACMD mkdir mega:/testing > /dev/null 2>&1
}

function silent() {
    $@ &> $OUT
}

function run() {
    echo
    echo Executing run $@
    $@ &> $OUT

    if [ $? -ne 0 ];
    then
        echo ${BASH_SOURCE[1]}:${BASH_LINENO[0]} FAIL: Executing failed
        cat $OUT
        exit 1
    fi
}


function run_fail() {
    echo
    echo Executing fail run $@
    $@ &> $OUT

    if [ $? -eq 0 ];
    then
        echo ${BASH_SOURCE[1]}:${BASH_LINENO[0]} FAIL: Executing failed - expects failure
        cat $OUT
        exit 1
    fi
}

function fail() {
    echo ${BASH_SOURCE[1]}:${BASH_LINENO[0]} FAIL: Test failed.. $@
    echo OUTPUT
    cat $OUT
    exit 1
}
