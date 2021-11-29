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

write_dm_tx_json_head() {
    out_file="$1"
    echo -e "{ \"before_test\":" > "${out_file}"
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
    echo -e "{ \"before_test\":" > "${out_file}"
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

    rm -f "${output}"
    echo "[" >> "${output}"
    get_sample  >> "${output}"
    while true; do
	echo ","  >> "${output}"
	get_sample >> "${output}"
	sleep 1
    done
}

setup_ssh_config() {
    cat > /root/.ssh/config <<EOF
Host guest
  HostName 10.1.0.128
  user pocuser
  ControlPath ~/.ssh/%r@%h:%p
  ControlMaster auto
  ControlPersist 10m
  SendEnv FIO_*
EOF
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
    FIO_rw="${1}"
    FIO_bs="${2}"
    FIO_nr_jobs="${3}"
    FIO_iodepth="${4}"

    out_dir="${FIO_rw}-${FIO_nr_jobs}jobs"
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

    echo "Starting job"
    export FIO_OUT_PATH
    export FIO_bs
    export FIO_rw
    export FIO_nr_jobs
    export FIO_iodepth

    sleep 1
    ssh guest "rm -rf ${FIO_OUT_PATH} && mkdir ${FIO_OUT_PATH}" || exit 1

    ssh guest "fio fio-template.job --output-format=json+,normal --output=${FIO_OUT_PATH}/result.json"
    scp -r guest "${FIO_OUT_PATH}" "${out_dir}"
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
    target_device=sdb

    format_disk || exit 1

    one_test write 256 1 1

}

main
# setup_ssh_config
# write_sys_stat_json_head test_sys.json
# write_sys_stat_json_tail test_sys.json
