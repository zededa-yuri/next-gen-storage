#!/bin/bash

rm -f /etc/resolv.conf
ln -s /run/systemd/resolve/resolv.conf /etc/
rm /root/first-boot.sh
