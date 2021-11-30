#!/bin/sh

get_dm_tx_json_body() {
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

    printf "\"%s\": %d" "${param}" "$(cat /sys/module/zfs/parameters/${param})"
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
	params="${params}" "${new_params}"
    fi

    printf "\"version\": \"%s\"" "$(cat /sys/module/zfs/version)"
    for opt in ${params}; do
	echo ","
	zfs_get_one_param_json "${opt}"
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
    echo ",\"after_test\":" >> "${out_file}"
    get_dm_tx_json_body >> "${out_file}"
    echo -e "}\n" >> "${out_file}"
}

write_sys_stat_json_head() {
    out_file="$1"
    echo -e "{ \"zfs\":" > "${out_file}"
    get_zfs_params_json >> "${out_file}"

    echo ",\"before_test\":" >> "${out_file}"
    get_sample >> "${out_file}"
}

write_sys_stat_json_tail() {
    out_file="$1"
    echo ",\"after_test\":" >> "${out_file}"
    get_sample >> "${out_file}"
    echo -e "}\n" >> "${out_file}"
}

get_sample()
{
    # Memory
    meminfo="$(cat /proc/meminfo)"
    memTotal=$(echo "${meminfo}" | egrep '^MemTotal:' | awk '{print $2}')
    memFree=$(echo "${meminfo}" | egrep '^MemFree:' | awk '{print $2}')
    memCached=$(echo "${meminfo}" | egrep '^Cached:' | awk '{print $2}')

    memUsed=$(($memTotal - $memFree))
    swapTotal=$(echo "${meminfo}" | egrep '^SwapTotal:' | awk '{print $2}')
    swapFree=$(echo "${meminfo}" | egrep '^SwapFree:' | awk '{print $2}')


    read CPUusage CPUsystem CPUidle << EOF
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
    trap "echo terminating monitor.. && echo ] >> ${output} && return 0" TERM
    trap exit 1 KILL

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
}


one_test_finish() {
    out_dir="$1"
    monitor_pid="$2"

    echo "exitting.."
    write_dm_tx_json_tail "${out_dir}"/zfs_dm_stats.json;
    write_sys_stat_json_tail "${out_dir}"/sys_stats.json;
    kill "${monitor_pid}"
    wait "${monitor_pid}"
}

one_test() {
    results_dir="${1}"
    FIO_rw="${2}"
    FIO_bs="${3}"
    FIO_nr_jobs="${4}"
    FIO_iodepth="${5}"

    test_name="${FIO_rw}-jobs${FIO_nr_jobs}-bs${FIO_bs}-iodepth${FIO_iodepth}"
    out_dir="${results_dir}/${test_name}"
    FIO_OUT_PATH="fio-output/${out_dir}-guest"

    zfs_trim || exit 1

    rm -rf "${out_dir}"
    mkdir "${out_dir}"
    write_dm_tx_json_head "${out_dir}"/zfs_dm_stats.json
    write_sys_stat_json_head "${out_dir}"/sys_stats.json;

    monitor "${out_dir}"/sys_stats_log.json&
    monitor_pid=$!
    sleep 2

    set -e

    trap "one_test_finish ${out_dir} ${monitor_pid}"  EXIT

    echo "Starting job ${out_dir}"
    export FIO_OUT_PATH
    export FIO_bs
    export FIO_rw
    export FIO_nr_jobs
    export FIO_iodepth

    sleep 1
    ssh guest "rm -rf ${FIO_OUT_PATH} && mkdir -p ${FIO_OUT_PATH}" || exit 1

    ssh guest "fio fio-template.job \
--output-format=json+,normal \
--output=${FIO_OUT_PATH}/result.json \
--group_reporting --eta-newline=1 \
> ${FIO_OUT_PATH}/fio-log.txt
"
    scp -r guest:"${FIO_OUT_PATH}" "${out_dir}"
    ssh guest "rm -rf ${out_dir}"
    echo "Done"
}

zfs_trim() {
    trim_done=0
    for i in $(seq 1 10); do
	if zpool trim persist -w; then
	    echo "Pool trimmed successfuly"
	    trim_done=1
	    break;
	fi
	echo "zpool trim failed, repeating after 1 sec"
	sleep 1;
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
    ssh guest "sudo umount /dev/${target_device}"
    ssh guest "sudo blkdiscard /dev/${target_device}" || exit 1
    ssh guest "sudo mkfs.ext4 /dev/${target_device}" || exit 1
    ssh guest "sudo mount /dev/${target_device} /mnt" || exit 1
    ssh guest "sudo chown pocuser:pocuser /mnt" || exit 1

    # Just in case to let zfs realized that blocks have been trimmed
    sync && sleep 2
}

main() {
    results_dir="${1}"
    target_device=sdb

    if ! command -v zpool; then
	echo "Please run"
	echo "    apk update && apk add zfs"
	exit 1
    fi

    if [ -d "${results_dir}" ]; then
	read -p "${results_dir} exists, remove? Y/n" yn
	case $yn in
            [Yy]* ) echo yes; break;;
            [Nn]* ) echo no; exit 1;;
            * ) echo "Please answer yes or no."; exit ;;
	esac
    fi

    format_disk || exit 1

    mkdir -p "${results_dir}"
    # results_dir rw bs jobs_nr iodepth
    one_test "${results_dir}" write 256k 1 1

    for load in write read randwrite randwrite trimwrite; do
	for nr_jobs in 2 4; do
	    for io_depth in 1 4 16; do
		one_test "${results_dir}" "${load}" 64k "${nr_jobs}" "${io_depth}"
	    done
	done
    done
}

#get_zfs_params_json
setup_ssh_config
main zfs_untuned
# setup_ssh_config
# write_sys_stat_json_head test_sys.json
# write_sys_stat_json_tail test_sys.json
# write_dm_tx_json_head test_dm_tx.json
# write_dm_tx_json_tail test_dm_tx.json
