package qemutmp

const QemuConfTemplate = `
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
  size = "{{.Memory}}"


[smp-opts]
  cpus = "{{.VCpus}}"
  sockets = "1"
  cores = "{{.VCpus}}"
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

const QemuUserData = `#cloud-config
password: "{{.Password}}"
chpasswd: { expire: False }
ssh_pwauth: True
`
