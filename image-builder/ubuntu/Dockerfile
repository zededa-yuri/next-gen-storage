FROM ubuntu:latest
ARG HOST_SRC_PATH
LABEL com.iximiuz-project="docker-to-linux"
RUN apt-get update -y
RUN DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends tzdata
RUN  apt-get --yes install \
  linux-image-virtual \
  systemd-sysv python3-pip iproute2 git\
  isc-dhcp-client openssh-server iputils-ping dnsutils \
  python3-configshell-fb libcurl4 libpixman-1-0

RUN pip3 install git+git://git.infradead.org/users/hch/nvmetcli.git

RUN echo "root:root" | chpasswd

RUN systemctl enable systemd-networkd

COPY wired.network /etc/systemd/network/wired.network
COPY ssh /root/.ssh
COPY fstab /etc/fstab
COPY first-boot.service /etc/systemd/system/first-boot.service
COPY first-boot.sh /root/first-boot.sh
RUN systemctl enable first-boot.service

COPY local-init.service /etc/systemd/system
RUN systemctl enable local-init.service

RUN touch /root/nvme-test.img && truncate -s 1G /root/nvme-test.img
RUN ln -s $HOST_SRC_PATH/virtual-host-init.sh /root/local-init.sh

