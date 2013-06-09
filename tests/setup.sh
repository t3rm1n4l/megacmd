#!/bin/bash

CONFIG="test_config.json"
sed "s/MEGA_PASSWD/$MEGA_PASSWD/;s/MEGA_USER/$MEGA_USER/" $CONFIG > t.json

