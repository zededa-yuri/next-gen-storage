#!/bin/bash
node_name="test_bench"
ipxe_cfg_path="ipxe.cfg"
packet_token=""
zedcloud_user="user@email.com"
zedcloud_token=""
zedcloud_project="project-name"
packet_instance_UUID=""
app_instance_name="test-app-fio"
cloid_init_cfg_path="cloud-init-config.yml"
packet_instance_IP="S167327F7186F22"
edge_app_image="bionic-cloud_img-bench"

metal device create \
           --hostname "${node_name}" \
           --project-id "e821eddc-7b3c-40b4-9960-0ce2d368fc26" \
           --facility ams1 \
           --plan t1.small.x86 \
           --operating-system custom_ipxe   \
           --userdata-file "${ipxe_cfg_path}"


sleep(180)

docker run --rm -itd -v "${PWD}":"${PWD}" --name zcli zededa/zcli
docker exec -it zcli zcli configure -T "${zedcloud_token}" --user="${zedcloud_user}" --server="zedcontrol.hummingbird.zededa.net" --output=json
docker exec -it zcli zcli login

# get ip addr
#metal device get --id "${packet_instance_UUID}" --output json > o.json
#packet_instance_IP="$(< o.json| jq -r '.ip_addresses[]| select(.address_family == 4) | select(.public == true) | .address')"
#echo "${packet_instance_IP}"



docker exec -it zcli zcli edge-node create "${node_name}" --title="${node_name}" --project="${zedcloud_project}" --onboarding-key="5d0767ee-0547-4569-b530-387e526f8cb9" --serial="${packet_instance_IP}" --model="ZedVirtual-4G" --network="eth0:management:defaultIPv4-net" --network="eth1:appshared:defaultIPv4-net"
docker exec -it zcli zcli edge-node activate "${node_name}"

sleep(180)

docker exec zcli zcli network-instance create "${node_name}"_net --kind=local --edge-node="${node_name}" --port=eth0 --ip-type=v4

docker exec -it zcli zcli edge-app-instance create "${app_instance_name}" \
                                --edge-app="${edge_app_image}" \
                                --edge-node="${node_name}" \
                                --title="${app_instance_name}" \
                                --description="Auto test" \
                                --network-instance=eth0:"${node_name}"_net \
                                --custom-configuration="${cloid_init_cfg_path}"

docker exec -it zcli zcli edge-app-instance start "${app_instance_name}"
