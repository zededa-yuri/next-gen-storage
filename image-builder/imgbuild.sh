#!/bin/bash

function export_ext_img() {
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

function get_ubuntu_tag_arg() {
    local distr="${1}"
    local release

    if [[ "${distr}" != "ubuntu" ]]; then
	return 0
    fi

    if ! grep Ubuntu /etc/lsb-release > /dev/null 2>&1; then
       return 0
    fi

    release="$(lsb_release -rs)"
    echo --build-arg TAG="${release}"
}

function main() {
    local output="${1:-ubuntu.img}"
    local distr="${2:-ubuntu}"

    local script_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
    local repo_root="$(realpath "${script_dir}"/..)"


    docker_cmd=(docker build "${script_dir}"/"${distr}" -t "${distr}"-img)
    docker_cmd+=(--build-arg HOST_SRC_PATH="${repo_root}")
    docker_cmd+=( $(get_ubuntu_tag_arg "${distr}") )

    "${docker_cmd[@]}"

    if (( $? != 0 )); then
	echo "Failed to build "${distr}"-img"
	exit 1
    fi

    export_ext_img "${output}" "${distr}"-img
}
main "$@"
