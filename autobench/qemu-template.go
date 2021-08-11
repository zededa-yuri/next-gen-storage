package main

const qemuConfTemplate = `
[drive "hd"]
  if = "none"
  file = "bionic-server-cloudimg-i386.img"
  format = "raw"

[device]
  driver = "intel-iommu"
  caching-mode = "on"

[device "scsi"]
  driver = "virtio-scsi-pci"
  bus = "pcie.0"
  addr = "0x7"

[device]
  driver = "scsi-hd"
  drive = "hd"

[device]
  driver = "vhost-scsi-pci"
  wwpn = "naa.500140508c1f5e99"
  bus = "pcie.0"
  addr = "0x08"

[rtc]
  base = "localtime"
  driftfix = "slew"

[global]
  driver = "kvm-pit"
  property = "lost_tick_policy"
  value = "delay"

[global]
  driver = "ICH9-LPC"
  property = "disable_s3"
  value = "1"

[global]
  driver = "ICH9-LPC"
  property = "disable_s4"
  value = "1"

[machine]
  type = "pc-q35-3.1"
  dump-guest-core = "off"
  accel = "kvm"
  vmport = "off"
  kernel-irqchip = "on"
  graphics = "off"

[memory]
  size = "512"

[smp-opts]
  cpus = "2"
  sockets = "1"
  cores = "2"
  threads = "1"

[realtime]
  mlock = "off"

[msg]
  timestamp = "on"

[chardev "ch0"]
  backend = "socket"
  path = "qemu.serial.socket"
  server = "on"
  wait = "off"
  logfile = "guest.log"

[chardev "charmonitor"]
  backend = "socket"
  path = "qemu.monitor.socket"
  server = "on"
  wait = "off"

[mon "charmonitor"]
  mode = "readline"
  chardev = "charmonitor"
`
