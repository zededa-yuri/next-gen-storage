# Auto-Benchmark tests util

This utility allows you to perform load performance testing with the ability to generate reports in the form of CSV tables and graphs based on JSON and log files FIO. It is also possible to test performance on virtual machines in 3 variations (in normal mode, with the creation and testing of LVM or ZFS logical volumes), and the ability to run testing on remote machines via an ssh connection.

## Building and configuring the host system

### Installing the necessary utilities

```bash
sudo apt-get install -y build-essential autoconf automake libtool gawk alien fakeroot dkms libblkid-dev uuid-dev libudev-dev libssl-dev zlib1g-dev libaio-dev libattr1-dev libelf-dev linux-headers-$(uname -r) python3 python3-dev python3-setuptools python3-cffi libffi-dev python3-packaging git libcurl4-openssl-dev targetcli-fb qemu-kvm qemu virt-manager virt-viewer libvirt-clients libvirt-daemon-system bridge-utils virtinst libvirt-daemon fio sysstat
```

Installing the latest version of Golang - [click here](https://go.dev/doc/install)

Installing OpenZfs - [click here](https://github.com/openzfs/zfs)

### Build and run

```bash
cd next-gen-storage/autobench
go build .
./autobench -h
```

## VM requirements and initial configuration

### Downloading and preparing the image for launch

You can take any img you need [here](https://cloud-images.ubuntu.com/bionic/current/) or [here](https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-i386.img)

>It is not necessary to use ubuntu images, but keep in mind that you will have to specify the necessary user/password options yourself and generate your own user-img with the desired user! (see help on launching ./autobench qemu -h)
>The downloaded image must be in 'img' format. QCOW2 format is invalid.

You can check that the image is in raw format with the command **qemu-img info**

Example:

```bash
qemu-img info bionic-server-cloudimg-i386.img

Output:
file format: qcow2
virtual size: 2.2 GiB (2361393152 bytes)
disk size: 336 MiB
cluster_size: 65536
Format specific information:
    compat: 0.10
    refcount bits: 16
```

From the example above, you can see that the image is in qcow2 format! To convert an image to IMG format, you need to run the following command:

```bash
qemu-img convert bionic-server-cloudimg-i386.img ~/next-gen-storage/autobench/bionic-server-cloudimg-i386.img
```

If you plan to run testing on a local disk in a virtual machine, then in this case it is necessary to expand the disk space on the virtual machine with the following command (Specify the size based on your needs):

```bash
qemu-img resize bionic-server-cloudimg-i386.img +10G
```

### First launch of the image to configure it

Trial run:

```bash
./autobench --size=1 qemu -p=8755
```

If the launch for the image is the first, autibench will fail the test with an error running the FIO command on the VM, BUT will not shutdown the VM so that you can configure it. If there are other kinds of problems, try to correct the situation based on the data obtained from the error.

Connecting to VM:

```bash
ssh -p 8755 ubuntu@localhost
```

### Installing fio and other necessary utilities

```bash
sudo apt-get update
sudo apt-get upgrade
wget <https://packages.ubuntu.com/bionic/i386/fio/download>
sudo dpkg -i fio_3.1-1_i386.deb
sudo apt --fix-broken install
sudo dpkg -i fio_3.1-1_i386.deb
sudo apt-get install sysstat
```

### Adding the required access rights to the user

```bash
sudo usermod -a -G disk,root ubuntu
```

You can check that the groups have been added with the following commands:

```bash
>groups ubuntu

Output:
ubuntu : ubuntu root adm disk dialout cdrom floppy sudo audio dip video plugdev lxd netdev

>id ubuntu

Output:
uid=1000(ubuntu) gid=1000(ubuntu) groups=1000(ubuntu),0(root),4(adm),6(disk),20(dialout),24(cdrom),25(floppy),27(sudo),29(audio),30(dip),44(video),46(plugdev),108(lxd),114(netdev)
```

After everything has been installed and configured, the virtual machine must be turned off and run test again with the options you need.

## Features and functionality

* Fio tests

* Generating CSV tables

* Plotting graphs based on CSV tables

### License

GNU General Public License v2.0
