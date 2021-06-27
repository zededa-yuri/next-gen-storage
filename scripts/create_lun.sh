#!/bin/bash

fileio_prefix=fileio_1
mkdir -p /sys/kernel/config/target/core/"${fileio_prefix}"/fileio
fpath=/root/scsi-test.img
if [ ! -f "${fpath}" ]; then
       touch "${fpath}"
       truncate -s 1G "${fpath}"
fi

size_bytes="$(stat --printf="%s" "${fpath}")"
echo "fd_dev_name="${fpath}",fd_dev_size="${size_bytes}",fd_buffered_io=1" > \
    /sys/kernel/config/target/core/"${fileio_prefix}"/fileio/control
echo 4096 > /sys/kernel/config/target/core/"${fileio_prefix}"/fileio/attrib/block_size

serial=000000000000000b
echo "${serial}" > \
    /sys/kernel/config/target/core/"${fileio_prefix}"/fileio/wwn/vpd_unit_serial

echo -n "test_fio" > \
    /sys/kernel/config/target/core/"${fileio_prefix}"/fileio/udev_path

mkdir -p /sys/kernel/config/target/vhost/naa."${serial}"/tpgt_1/lun/lun_0

echo 1 > /sys/kernel/config/target/core/"${fileio_prefix}"/fileio/enable

serial_nexus=$(printf '%x' $(( 16#$serial + 1 )))
echo -n "naa.${serial_nexus}"  > \
    /sys/kernel/config/target/vhost/naa."${serial}"/tpgt_1/nexus

ln -s /sys/kernel/config/target/core/"${fileio_prefix}"/fileio/  /sys/kernel/config/target/vhost/naa."${serial}"/tpgt_1/lun/lun_0

