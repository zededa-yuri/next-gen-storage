Next Gen Storage project
========================

The sole purpose of this repo is to provide a bit of a structure for
next-gen-storage project. Just to provide a way to synchronize
different repositories (e.g. linux, qemu, potentially open-zfs) and a
storage for necessary config files and helper scripts.

Links to submodules are relative, so feel free to clone this
repository to your github/gitlab/whatever. You just need to clone
`linux` and `qemu` repositories as well and `clone --recursive` will
automagically pull submodules from your repositories.

How to build and run
====================

Assuming that you have `ubuntu-1.img` and `alpine.qcow2` in the root
folder of this repository. And alpine.qcow2 has an entry in fstab

.. code-block:: console

   src  /path-to-your-src-on-your-host/ 9p trans=virtio,version=9p2000.L,posixacl,msize=104857601,cache=loose

Launcher scripts are assuming that your folder with source is mounted
via 9pfs to identical path in the vitual machine. So there are no
headaches with resolving paths to sources when using gdb.

.. code-block:: bash

   git clone --recursive git@github.com:itmo-eve/next-gen-storage.git

   cd next-gen-storage/linux
   cp ../configs/linux-config .config
   make oldconfig
   make

   mkdir ../qemu/build && cd ../qemu/build
   ../configure --target-list=x86_64-softmmu --enable-virtfs --extra-cflags='-O0'
   make
   cd ../../

   #Build virtual host image
   image-builder/prepare.sh image-builder/ubuntu
   image-builder/imgbuild.sh ubuntu-1.img

   #Run virtual host
   ./qemu-run.py
   login/pass root:root

   # from another terminal
   ssh -p 5551 ubuntu@localhost
   nvmetcli restore /home/yuri/src/next-gen-storage/configs/vhost.json
   /users/yuri/src/next-gen-storage/nested-run.py
