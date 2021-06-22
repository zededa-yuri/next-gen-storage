#!/bin/bash

function main() {
    local output="$1"
    mkdir -p "${output}"/ssh
    ssh-keygen -f "${output}"/ssh/id_rsa.pub -N ""
    cp "${HOME}"/.ssh/id_rsa.pub "${output}"/ssh/authorized_keys

    echo  "src  ${HOME}/src 9p trans=virtio,version=9p2000.L,posixacl,msize=104857601,cache=loose" > "${output}"/fstab
}

main "$@"
