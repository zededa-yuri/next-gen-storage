#!/bin/bash

function main() {
    local output="$1"
    local docker_image="$2"
    if test -f "${output}"; then
	mv -f "${output}" prev-"${output}"
    fi

    touch "${output}"
    truncate "${output}" --size=20G
    mkfs.ext4 "${output}"

    local container="$(docker run -d "${docker_image}"  /bin/true)"
    docker export -o "${output}".tar "${container}"
 
    mkdir -p mnt
    sudo mount -o loop "${output}" mnt
    sudo tar -xf "${output}".tar -C mnt
    sudo umount mnt
    docker container rm "${container}"
}

main "$@"
