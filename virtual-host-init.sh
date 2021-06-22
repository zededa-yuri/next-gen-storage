#!/bin/bash

function enable_kernel_debug {
    echo 8 8 8 8 > /proc/sys/kernel/printk
    echo "file drivers/nvme/* +p" > /sys/kernel/debug/dynamic_debug/control 
    echo "file drivers/target/* +p" > sudo tee /sys/kernel/debug/dynamic_debug/control 
}

enable_kernel_debug
