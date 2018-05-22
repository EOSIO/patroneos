#!/bin/bash

haproxy -f /usr/local/etc/haproxy/haproxy.cfg
fail2ban-client -b start
patroneosd -configFile /etc/patroneos/config.json -mode fail2ban-relay &

tail -f /var/log/patroneosd.log /var/log/fail2ban.log
