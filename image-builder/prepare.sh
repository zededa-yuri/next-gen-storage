#!/bin/bash

function main() {
    local output="$1"
    local script_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
    local repo_root="$(realpath "${script_dir}"/..)"

    mkdir -p "${output}"/ssh
    ssh-keygen -f "${output}"/ssh/id_rsa.pub -N ""
    cp "${HOME}"/.ssh/id_rsa.pub "${output}"/ssh/authorized_keys

    echo  "src  ${HOME}/src 9p trans=virtio,version=9p2000.L,posixacl,msize=104857601,cache=loose" > "${output}"/fstab
    ln -s "${repo_root}"/virt-host-init.sh "${output}"/local-init.sh
}

main "$@"
