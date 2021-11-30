#!/bin/bash


get_dm_tx_json_body() {
    # shellcheck disable=SC2016
    awk_script='
{
    if (NR==1) {
        print "\"first_line\": \"" $0 "\","
    }

    if (NR>3) {
        print str ",";
    }
    str="\"" $1 "\": " $3
}
END {
    print str;
}
'

    echo "{"
    echo "\"date\" : \"$(date -Iseconds)\","
    awk "${awk_script}" /proc/spl/kstat/zfs/dmu_tx
    echo "}"
}

zfs_get_one_param_json() {
    param="$1"

    val="$(cat /sys/module/zfs/parameters/"${param}")" || return 1
    printf "\"%s\": %d" "${param}" "${val}"
}
get_zfs_params_json() {
    echo "{"
    params="zfs_compressed_arc_enabled \
zfs_vdev_min_auto_ashift \
zvol_request_sync \
zfs_arc_min \
zfs_arc_max \
zfs_vdev_aggregation_limit_non_rotating \
zfs_vdev_async_write_active_min_dirty_percent \
zfs_vdev_async_write_active_max_dirty_percent \
zfs_delay_min_dirty_percent \
zfs_delay_scale \
zfs_dirty_data_max \
zfs_dirty_data_sync_percent \
zfs_prefetch_disable \
zfs_vdev_sync_read_min_active \
zfs_vdev_sync_read_max_active \
zfs_vdev_sync_write_min_active \
zfs_vdev_sync_write_max_active \
zfs_vdev_async_read_min_active \
zfs_vdev_async_read_max_active \
zfs_vdev_async_write_min_active \
zfs_vdev_async_write_max_active \
"

    new_params="zfs_smoothing_scale zfs_write_smoothing"
    if [ -f /sys/module/zfs/parameters/zfs_smoothing_scale ]; then
	params="${params} ${new_params}"
    fi

    printf "\"version\": \"%s\"" "$(cat /sys/module/zfs/version)"
    for opt in ${params}; do
	echo ","
	zfs_get_one_param_json "${opt}" || return 1
    done
    echo -e "\n}"
}

write_dm_tx_json_head() {
    out_file="$1"
    echo -e "{\"before_test\":" > "${out_file}"
    get_dm_tx_json_body >> "${out_file}"
}

write_dm_tx_json_tail() {
    out_file="$1"
    {
	echo ",\"after_test\":"
	get_dm_tx_json_body
	echo -e "}\n"
    } >> "${out_file}"
}

get_sample_with_fragmentation() {
    sample="$(get_sample)" || return 1
    frag="$(zpool get -Hp fragmentation persist)" || return 1
    frag="$(awk '{print $3}' <<< "${frag}")" || return 1

    json="$(jq --arg frag "${frag}" '. | .fragmentation=$frag' <<< "${sample}")" || return 1

    echo "${json}"
}

write_sys_stat_json_head() {
    out_file="$1"

    json="$(get_zfs_params_json)" || return 1
    sample="$(get_sample_with_fragmentation)" || return 1

    json="$(jq '{zfs:.}' <<< "${json}")" || return 1

    json="$(jq --argjson before "${sample}" '. | .before_test=$before' <<< "${json}")" || return 1

    cat <<<"${json}" > "${out_file}"

    # get_sample >> "${out_file}"
}

write_sys_stat_json_tail() {
    out_file="$1"
    json="$(<"${out_file}")" || return 1
    sample="$(get_sample_with_fragmentation)" || return 1

    json="$(jq --argjson after "${sample}" '. | .after_test=$after' <<< "${json}")" || return 1
    cat <<<"${json}" > "${out_file}"
}

get_sample()
{
    # Memory
    meminfo="$(cat /proc/meminfo)"
    memTotal=$(echo "${meminfo}" | grep -E '^MemTotal:' | awk '{print $2}')
    memFree=$(echo "${meminfo}" | grep -E '^MemFree:' | awk '{print $2}')
    memCached=$(echo "${meminfo}" | grep -E '^Cached:' | awk '{print $2}')

    memUsed=$((memTotal - memFree))
    swapTotal=$(echo "${meminfo}" | grep -E '^SwapTotal:' | awk '{print $2}')
    swapFree=$(echo "${meminfo}" | grep -E '^SwapFree:' | awk '{print $2}')


    read -r CPUusage CPUsystem CPUidle << EOF
$(grep 'cpu ' /proc/stat | awk '{print $2 " " $4 " " $5}')
EOF

    # Final result in JSON
    JSON="
    {
  \"date\" : \"$(date -Iseconds)\",
  \"memory\":
  {
    \"MemTotal\": $memTotal,
    \"MemFree\": $memUsed,
    \"Cached\": $memCached,
    \"swapTotal\": $swapTotal,
    \"swapFree\": $swapFree
  },
  \"cpu\":
  {
    \"usage\": $CPUusage,
    \"system\": $CPUsystem,
    \"idle\": $CPUidle
  }
}"

    echo -n "$JSON"
}


monitor() {
    local output="$1"

    echo "Starting monitor.."
    #trap "echo ] >> ${output}" SIGINT

    # shellcheck disable=SC2064
    trap "echo terminating monitor.. && echo ] >> ${output} && return 0" SIGTERM

    rm -f "${output}"
    if ! touch "${output}"; then
	echo "${output} is not accessible"
	exit 1
    fi

    echo "[" >> "${output}"
    get_sample  >> "${output}"
    while true; do
	echo ","  >> "${output}"
	get_sample >> "${output}"
	sleep 1
    done
}

setup_ssh_config() {
    if [ -f /root/.ssh/id_rsa.pub ]; then
	return
    fi

    cat > /root/.ssh/config <<EOF
Host guest
  HostName 10.1.0.128
  user pocuser
  ControlPath ~/.ssh/%r@%h:%p
  ControlMaster auto
  ControlPersist 10m
  SendEnv FIO_*
EOF

    cp ssh-keys/* ~/.ssh/

    cat >> /root/.ssh/authorized_keys << EOF
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCoEmuzL43ZkQ2s4KBjJj1uanzwnmPAHhoI3x3BvGbRIc5vHnl3ZUu7QNtiu5uRnjN9cjt1ZpQfMjpSLdTLyMk6Q7CLSTZ69PtDjp74+CxkMnsMQj6BPFJdVrQtJayynyqOJsF9sbiH3cpV7fgBGOrLjonVtQYqk6I7Ia//G02oH9f4g9gV8Z8xU6BI6qgEZ3GVvN0Od6vtn+M7yVS8gwqry6edGpwTqf9DizNXKhIVHySbKxmrWOVBD2xa66pm8vwvPccTMYPY6WTt57Q/bGTuSaewSOdDfvOJB1YgVR+cyvhWsXJI1s27HDeYXrippN4JQhnC+7CgLN87CAvhE8mdA6mwngmCUOcuguX3KkY9INUJmHYy/j/zxO+Xh0NWThJP0ddtgMq/ewO2f2nHEuBd87UmuPwhYPlwaX3gLWzpqNFurw9IASDRVj5E59RuO06GtLds0rw6qB76lFymoVArb6oyNoSQsJ8crbPgl4qhgjqwiNouZqCQ051SQoc/DgM= yuri@Yuris-MacBook-Pro-16.local
EOF
}


one_test() {
    results_dir="${1}"
    template="${2}"
    FIO_rw="${3}"
    FIO_bs="${4}"
    FIO_nr_jobs="${5}"
    FIO_iodepth="${6}"

    test_name="${FIO_rw}-jobs${FIO_nr_jobs}-bs${FIO_bs}-iodepth${FIO_iodepth}"
    out_dir="${results_dir}/${test_name}"
    FIO_OUT_PATH="fio-output/${out_dir}-guest"

    zfs_trim || return 1

    rm -rf "${out_dir}"
    mkdir "${out_dir}"
    write_dm_tx_json_head "${out_dir}"/zfs_dm_stats.json
    write_sys_stat_json_head "${out_dir}"/sys_stats.json;

    monitor "${out_dir}"/sys_stats_log.json&
    monitor_pid=$!
    sleep 2

    echo "Starting job ${out_dir}"
    export FIO_OUT_PATH
    export FIO_bs
    export FIO_rw
    export FIO_nr_jobs
    export FIO_iodepth

    sleep 1
    # shellcheck disable=SC2029
    if ! ssh guest "rm -rf ${FIO_OUT_PATH} && mkdir -p ${FIO_OUT_PATH}"; then
	echo "failed creating results dir on the guest"
	return 1
    fi

    # shellcheck disable=SC2029
    local fio_command="fio ${template} \
--output-format=json+,normal \
--output=${FIO_OUT_PATH}/result.json \
--group_reporting --eta-newline=1 \
> ${FIO_OUT_PATH}/fio-log.txt
"
    # shellcheck disable=SC2029
    if ! ssh guest "${fio_command}"; then
	echo "Failed runnning FIO"
	kill "${monitor_pid}" && wait "${monitor_pid}"
	exit 1
    fi

    # shellcheck disable=SC2029
    if ! scp -r guest:"${FIO_OUT_PATH}" "${out_dir}"; then
	echo "failed downloading results"
	kill "${monitor_pid}" && wait "${monitor_pid}"
	return 1
    fi

    # shellcheck disable=SC2029
    ssh guest "rm -rf ${FIO_OUT_PATH}"
    kill "${monitor_pid}"
    write_dm_tx_json_tail "${out_dir}"/zfs_dm_stats.json;
    write_sys_stat_json_tail "${out_dir}"/sys_stats.json;
    wait "${monitor_pid}"
    echo "Done"
}

zfs_trim() {
    trim_done=0
    echo "attempting to trim the pool.."
    while true; do
	if zpool trim persist -w; then
	    echo "Pool trimmed successfuly"
	    trim_done=1
	    break;
	fi
	echo "zpool trim failed, repeating after 30 sec"
	sleep 30;
    done

    if [ "${trim_done}" = "0" ]; then
	echo "Giving up issuing zpool trim"
	return 1
    fi
}

format_disk() {
    # Umount can legitemaly fail, if it wasn't mount on the first
    # place
    echo "Cleaning target disk.."
    # shellcheck disable=SC2029
    ssh guest "sudo umount /dev/${target_device}"
    # shellcheck disable=SC2029
    ssh guest "sudo blkdiscard /dev/${target_device}" || return 1
    # shellcheck disable=SC2029
    ssh guest "sudo mkfs.ext4 /dev/${target_device}" || return 1
    # shellcheck disable=SC2029
    ssh guest "sudo mount /dev/${target_device} /mnt" || return 1
    ssh guest "sudo chown pocuser:pocuser /mnt" || return 1

    # Just in case to let zfs realized that blocks have been trimmed
    sync && sleep 2
}

main() {
    results_dir="${1}"
    target_device=sdb

    if ! command -v zpool; then
	echo "installing dependencies"
	apk update && apk add zfs rsync jq
    fi

    if [ -d "${results_dir}" ]; then
	# shellcheck disable=SC2029
	read -r -p "${results_dir} exists, remove? Y/n" yn
	case $yn in
            [Yy]* ) echo yes; rm -rf "${results_dir}" ;;
            [Nn]* ) echo no; exit 1;;
            * ) echo "Please answer yes or no."; exit ;;
	esac
    fi

    ssh guest "pkill fio"

    trap "ssh guest 'pkill fio'" SIGINT

    mkdir -p "${results_dir}"

    format_disk || exit 1
    # results_dir data_size rw bs jobs_nr iodepth
    one_test "${results_dir}" fio-place-data.job write 256k 1 1 || exit 1


    for load in read randread write randwrite trimwrite; do
	for nr_jobs in 1 2; do
	    for io_depth in 1 2 4 ; do
		one_test "${results_dir}" fio-template.job "${load}" 64k "${nr_jobs}" "${io_depth}" || exit 1
	    done
	done
    done
}

#get_zfs_params_json
setup_ssh_config
main zfs_untuned_p4
# setup_ssh_config
# get_sample_with_fragmentation
# write_sys_stat_json_head test_sys.json || exit 1
# write_sys_stat_json_tail test_sys.json || exit 1
# write_dm_tx_json_head test_dm_tx.json
# write_dm_tx_json_tail test_dm_tx.json
