EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/
EVENTS_FILE_NAME=$(date +%s%3N)
STARTTIME=$(date)
STARTTIME_FORMATTED=$(date +"%F %T.%3N")
ENDTIME_FORMATTED=$(date +"%F %T.%3N")
CGROUP_VERSION=$(stat -fc %T /sys/fs/cgroup)

CGROUP="/sys/fs/cgroup"
CSLICE=$(systemctl show containerd -p Slice | cut -d= -f2)
KSLICE=$(systemctl show kubelet -p Slice | cut -d= -f2)

if [ "$CGROUP_VERSION" = "cgroup2fs" ]; then

    cgroup_cpu_pressure=$(cat ${CGROUP}/cpu.pressure)
    cgroup_memory_pressure=$(cat ${CGROUP}/memory.pressure)
    cgroup_io_pressure=$(cat ${CGROUP}/io.pressure)

    cgroup_cpu_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $cgroup_cpu_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $cgroup_cpu_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $cgroup_cpu_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $cgroup_cpu_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $cgroup_cpu_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $cgroup_cpu_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $cgroup_cpu_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $cgroup_cpu_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    cgroup_memory_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $cgroup_memory_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $cgroup_memory_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $cgroup_memory_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $cgroup_memory_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $cgroup_memory_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $cgroup_memory_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $cgroup_memory_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $cgroup_memory_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    cgroup_io_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $cgroup_io_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $cgroup_io_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $cgroup_io_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $cgroup_io_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $cgroup_io_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $cgroup_io_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $cgroup_io_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $cgroup_io_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    cgroup_cpu_pressures=$(echo $cgroup_cpu_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    cgroup_memory_pressures=$(echo $cgroup_memory_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    cgroup_io_pressures=$(echo $cgroup_io_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

    cgroup_pressure=$( jq -n \
    --argjson CPU_PRESSURE "$(echo $cgroup_cpu_pressures)" \
    --argjson MEMORY_PRESSURE "$(echo $cgroup_memory_pressures)" \
    --argjson IO_PRESSURE "$(echo $cgroup_io_pressures)" \
    '{ CPUPressure: $CPU_PRESSURE, MemoryPressure: $MEMORY_PRESSURE, IOPressure: $IO_PRESSURE } | tostring'
    )

    SYSTEMSLICE="${CGROUP}//system.slice"
    system_slice_cpu_pressure=$(cat $SYSTEMSLICE/cpu.pressure)
    system_slice_memory_pressure=$(cat $SYSTEMSLICE/memory.pressure)
    system_slice_io_pressure=$(cat $SYSTEMSLICE/io.pressure)

    system_slice_cpu_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $system_slice_cpu_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $system_slice_cpu_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $system_slice_cpu_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $system_slice_cpu_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $system_slice_cpu_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $system_slice_cpu_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $system_slice_cpu_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $system_slice_cpu_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    system_slice_memory_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $system_slice_memory_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $system_slice_memory_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $system_slice_memory_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $system_slice_memory_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $system_slice_memory_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $system_slice_memory_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $system_slice_memory_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $system_slice_memory_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    system_slice_io_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $system_slice_io_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $system_slice_io_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $system_slice_io_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $system_slice_io_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $system_slice_io_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $system_slice_io_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $system_slice_io_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $system_slice_io_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    system_slice_cpu_pressures=$(echo $system_slice_cpu_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    system_slice_memory_pressures=$(echo $system_slice_memory_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    system_slice_io_pressures=$(echo $system_slice_io_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

    system_slice_pressure=$( jq -n \
    --argjson CPU_PRESSURE "$(echo $system_slice_cpu_pressures)" \
    --argjson MEMORY_PRESSURE "$(echo $system_slice_memory_pressures)" \
    --argjson IO_PRESSURE "$(echo $system_slice_io_pressures)" \
    '{ CPUPressure: $CPU_PRESSURE, MemoryPressure: $MEMORY_PRESSURE, IOPressure: $IO_PRESSURE } | tostring'
    )

    AZURESLICE="${CGROUP}/azure.slice"
    azure_slice_cpu_pressure=$(cat $AZURESLICE/cpu.pressure)
    azure_slice_memory_pressure=$(cat $AZURESLICE/memory.pressure)
    azure_slice_io_pressure=$(cat $AZURESLICE/io.pressure)

    azure_slice_cpu_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $azure_slice_cpu_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $azure_slice_cpu_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $azure_slice_cpu_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $azure_slice_cpu_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $azure_slice_cpu_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $azure_slice_cpu_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $azure_slice_cpu_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $azure_slice_cpu_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    azure_slice_memory_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $azure_slice_memory_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $azure_slice_memory_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $azure_slice_memory_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $azure_slice_memory_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $azure_slice_memory_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $azure_slice_memory_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $azure_slice_memory_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $azure_slice_memory_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    azure_slice_io_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $azure_slice_io_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $azure_slice_io_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $azure_slice_io_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $azure_slice_io_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $azure_slice_io_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $azure_slice_io_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $azure_slice_io_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $azure_slice_io_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    azure_slice_cpu_pressures=$(echo $azure_slice_cpu_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    azure_slice_memory_pressures=$(echo $azure_slice_memory_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    azure_slice_io_pressures=$(echo $azure_slice_io_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

    azure_slice_pressure=$( jq -n \
    --argjson CPU_PRESSURE "$(echo $azure_slice_cpu_pressures)" \
    --argjson MEMORY_PRESSURE "$(echo $azure_slice_memory_pressures)" \
    --argjson IO_PRESSURE "$(echo $azure_slice_io_pressures)" \
    '{ CPUPressure: $CPU_PRESSURE, MemoryPressure: $MEMORY_PRESSURE, IOPressure: $IO_PRESSURE } | tostring'
    )

    KUBEPODSSLICE="${CGROUP}/kubepods.slice"
    kubepods_slice_cpu_pressure=$(cat $KUBEPODSSLICE/cpu.pressure)
    kubepods_slice_memory_pressure=$(cat $KUBEPODSSLICE/memory.pressure)
    kubepods_slice_io_pressure=$(cat $KUBEPODSSLICE/io.pressure)

    kubepods_slice_cpu_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $kubepods_slice_cpu_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $kubepods_slice_cpu_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $kubepods_slice_cpu_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $kubepods_slice_cpu_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $kubepods_slice_cpu_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $kubepods_slice_cpu_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $kubepods_slice_cpu_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $kubepods_slice_cpu_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    kubepods_slice_memory_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $kubepods_slice_memory_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $kubepods_slice_memory_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $kubepods_slice_memory_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $kubepods_slice_memory_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $kubepods_slice_memory_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $kubepods_slice_memory_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $kubepods_slice_memory_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $kubepods_slice_memory_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    kubepods_slice_io_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $kubepods_slice_io_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $kubepods_slice_io_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $kubepods_slice_io_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $kubepods_slice_io_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $kubepods_slice_io_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $kubepods_slice_io_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $kubepods_slice_io_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $kubepods_slice_io_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    kubepods_slice_cpu_pressures=$(echo $kubepods_slice_cpu_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    kubepods_slice_memory_pressures=$(echo $kubepods_slice_memory_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    kubepods_slice_io_pressures=$(echo $kubepods_slice_io_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

    kubepods_slice_pressure=$( jq -n \
    --argjson CPU_PRESSURE "$(echo $kubepods_slice_cpu_pressures)" \
    --argjson MEMORY_PRESSURE "$(echo $kubepods_slice_memory_pressures)" \
    --argjson IO_PRESSURE "$(echo $kubepods_slice_io_pressures)" \
    '{ CPUPressure: $CPU_PRESSURE, MemoryPressure: $MEMORY_PRESSURE, IOPressure: $IO_PRESSURE } | tostring'
    )

    KUBELETSERVICE="${CGROUP}/${KSLICE}/kubelet.service"
    kubelet_service_cpu_pressure=$(cat $KUBELETSERVICE/cpu.pressure)
    kubelet_service_memory_pressure=$(cat $KUBELETSERVICE/memory.pressure)
    kubelet_service_io_pressure=$(cat $KUBELETSERVICE/io.pressure)

    kubelet_service_cpu_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $kubelet_service_cpu_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $kubelet_service_cpu_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $kubelet_service_cpu_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $kubelet_service_cpu_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $kubelet_service_cpu_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $kubelet_service_cpu_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $kubelet_service_cpu_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $kubelet_service_cpu_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    kubelet_service_memory_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $kubelet_service_memory_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $kubelet_service_memory_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $kubelet_service_memory_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $kubelet_service_memory_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $kubelet_service_memory_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $kubelet_service_memory_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $kubelet_service_memory_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $kubelet_service_memory_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    kubelet_service_io_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $kubelet_service_io_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $kubelet_service_io_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $kubelet_service_io_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $kubelet_service_io_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $kubelet_service_io_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $kubelet_service_io_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $kubelet_service_io_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $kubelet_service_io_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    kubelet_service_cpu_pressures=$(echo $kubelet_service_cpu_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    kubelet_service_memory_pressures=$(echo $kubelet_service_memory_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    kubelet_service_io_pressures=$(echo $kubelet_service_io_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

    kubelet_service_pressure=$( jq -n \
    --argjson CPU_PRESSURE "$(echo $kubelet_service_cpu_pressures)" \
    --argjson MEMORY_PRESSURE "$(echo $kubelet_service_memory_pressures)" \
    --argjson IO_PRESSURE "$(echo $kubelet_service_io_pressures)" \
    '{ CPUPressure: $CPU_PRESSURE, MemoryPressure: $MEMORY_PRESSURE, IOPressure: $IO_PRESSURE } | tostring'
    )

    CONTAINERDSERVICE="${CGROUP}/${CSLICE}/containerd.service"
    containerd_service_cpu_pressure=$(cat $CONTAINERDSERVICE/cpu.pressure)
    containerd_service_memory_pressure=$(cat $CONTAINERDSERVICE/memory.pressure)
    containerd_service_io_pressure=$(cat $CONTAINERDSERVICE/io.pressure)

    containerd_service_cpu_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $containerd_service_cpu_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $containerd_service_cpu_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $containerd_service_cpu_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $containerd_service_cpu_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $containerd_service_cpu_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $containerd_service_cpu_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $containerd_service_cpu_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $containerd_service_cpu_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    containerd_service_memory_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $containerd_service_memory_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $containerd_service_memory_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $containerd_service_memory_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $containerd_service_memory_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $containerd_service_memory_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $containerd_service_memory_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $containerd_service_memory_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $containerd_service_memory_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    containerd_service_io_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $containerd_service_io_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $containerd_service_io_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $containerd_service_io_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $containerd_service_io_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $containerd_service_io_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $containerd_service_io_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $containerd_service_io_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $containerd_service_io_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    containerd_service_cpu_pressures=$(echo $containerd_service_cpu_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    containerd_service_memory_pressures=$(echo $containerd_service_memory_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    containerd_service_io_pressures=$(echo $containerd_service_io_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

    containerd_service_pressure=$( jq -n \
    --argjson CPU_PRESSURE "$(echo $containerd_service_cpu_pressures)" \
    --argjson MEMORY_PRESSURE "$(echo $containerd_service_memory_pressures)" \
    --argjson IO_PRESSURE "$(echo $containerd_service_io_pressures)" \
    '{ CPUPressure: $CPU_PRESSURE, MemoryPressure: $MEMORY_PRESSURE, IOPressure: $IO_PRESSURE } | tostring'
    )

    cgroup_pressure=$(echo $system_slice_pressure | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    system_slice_pressure=$(echo $system_slice_pressure | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    azure_slice_pressure=$(echo $azure_slice_pressure | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    kubepods_slice_pressure=$(echo $kubepods_slice_pressure | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    kubelet_service_pressure=$(echo $kubelet_service_pressure | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    containerd_service_pressure=$(echo $containerd_service_pressure | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

    pressure_string=$( jq -n \
    --argjson CGROUP "$(echo $cgroup_pressure)" \
    --argjson SYSTEMSLICE "$(echo $system_slice_pressure)" \
    --argjson AZURESLICE "$(echo $azure_slice_pressure)" \
    --argjson KUBEPODSSLICE "$(echo $kubepods_slice_pressure)" \
    --argjson KUBELETSERVICE "$(echo $kubelet_service_pressure)" \
    --argjson CONTAINERDSERVICE "$(echo $containerd_service_pressure)" \
    '{ cgroup_pressure: $CGROUP, system_slice_pressure: $SYSTEMSLICE, azure_slice_pressure: $AZURESLICE, kubepods_slice_pressure: $KUBEPODSSLICE, kubelet_service_pressure: $KUBELETSERVICE, containerd_service_pressure: $CONTAINERDSERVICE } | tostring'
    )

    pressure_string=$(echo $pressure_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

    memory_string=$( jq -n \
        --arg SYSTEM_SLICE_MEMORY "$(($(cat ${CGROUP}/system.slice/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/system.slice/memory.stat | awk '/^anon /{print $2}')))" \
        --arg AZURE_SLICE_MEMORY "$(($(cat ${CGROUP}/azure.slice/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/azure.slice/memory.stat | awk '/^anon /{print $2}')))" \
        --arg KUBEPODS_SLICE_MEMORY "$(($(cat ${CGROUP}/kubepods.slice/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/kubepods.slice/memory.stat | awk '/^anon /{print $2}')))" \
        --arg USER_SLICE_MEMORY "$(($(cat ${CGROUP}/user.slice/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/user.slice/memory.stat | awk '/^anon /{print $2}')))" \
        --arg CONTAINERD_MEMORY "$(($(cat ${CGROUP}/${CSLICE}/containerd.service/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/${CSLICE}/containerd.service/memory.stat | awk '/^anon /{print $2}')))" \
        --arg KUBELET_MEMORY "$(($(cat ${CGROUP}/${KSLICE}/kubelet.service/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/${KSLICE}/kubelet.service/memory.stat | awk '/^anon /{print $2}')))" \
        --arg EMPLOYED_MEMORY "$(($(cat ${CGROUP}/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/memory.stat | awk '/^anon /{print $2}')))" \
        --arg CAPACITY_MEMORY "$(grep MemTotal /proc/meminfo | awk '{print $2}')" \
        --arg KUBEPODS_CGROUP_MEMORY_MAX "$(cat ${CGROUP}/kubepods.slice/memory.max)" \
        '{ system_slice_memory: $SYSTEM_SLICE_MEMORY, azure_slice_memory: $AZURE_SLICE_MEMORY, kubepods_slice_memory: $KUBEPODS_SLICE_MEMORY, user_slice_memory: $USER_SLICE_MEMORY, containerd_service_memory: $CONTAINERD_MEMORY, kubelet_service_memory: $KUBELET_MEMORY, cgroup_memory: $EMPLOYED_MEMORY, cgroup_capacity_memory: $CAPACITY_MEMORY, kubepods_max_memory: $KUBEPODS_CGROUP_MEMORY_MAX } | tostring'
    )

    memory_string=$(echo $memory_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

    message_string=$( jq -n \
        --arg CGROUPV "${CGROUP_VERSION}" \
        --argjson MEMORY "$(echo $memory_string)" \
        --argjson PRESSURE "$(echo $pressure_string)" \
        '{ CgroupVersion: $CGROUPV, Memory: $MEMORY, Pressure: $PRESSURE } | tostring'
    )
    
elif [ "$CGROUP_VERSION" = "tmpfs" ]; then

    memory_string=$( jq -n \
        --arg SYSTEM_SLICE_MEMORY "$(($(cat ${CGROUP}/system.slice/memory.stat | awk '/^cache /{print $2}') + $(cat ${CGROUP}/system.slice/memory.stat | awk '/^rss /{print $2}')))" \
        --arg AZURE_SLICE_MEMORY "$(($(cat ${CGROUP}/azure.slice/memory.stat | awk '/^cache /{print $2}') + $(cat ${CGROUP}/azure.slice/memory.stat | awk '/^rss /{print $2}')))" \
        --arg KUBEPODS_SLICE_MEMORY "$(($(cat ${CGROUP}/kubepods.slice/memory.stat | awk '/^cache /{print $2}') + $(cat ${CGROUP}/kubepods.slice/memory.stat | awk '/^rss /{print $2}')))" \
        --arg USER_SLICE_MEMORY "$(($(cat ${CGROUP}/user.slice/memory.stat | awk '/^cache /{print $2}') + $(cat ${CGROUP}/user.slice/memory.stat | awk '/^rss /{print $2}')))" \
        --arg CONTAINERD_MEMORY "$(($(cat ${CGROUP}/${CSLICE}/containerd.service/memory.stat | awk '/^cache /{print $2}') + $(cat ${CGROUP}/${CSLICE}/containerd.service/memory.stat | awk '/^rss /{print $2}')))" \
        --arg KUBELET_MEMORY "$(($(cat ${CGROUP}/${KSLICE}/kubelet.service/memory.stat | awk '/^cache /{print $2}') + $(cat ${CGROUP}/${KSLICE}/kubelet.service/memory.stat | awk '/^rss /{print $2}')))" \
        --arg EMPLOYED_MEMORY "$(($(cat ${CGROUP}/memory.stat | awk '/^cache /{print $2}') + $(cat ${CGROUP}/memory.stat | awk '/^rss /{print $2}')))" \
        --arg CAPACITY_MEMORY "$(grep MemTotal /proc/meminfo | awk '{print $2}')" \
        --arg KUBEPODS_CGROUP_MEMORY_MAX "$(cat ${CGROUP}/kubepods.slice/memory.max)" \
        '{ system_slice_memory: $SYSTEM_SLICE_MEMORY, azure_slice_memory: $AZURE_SLICE_MEMORY, kubepods_slice_memory: $KUBEPODS_SLICE_MEMORY, user_slice_memory: $USER_SLICE_MEMORY, containerd_service_memory: $CONTAINERD_MEMORY, kubelet_service_memory: $KUBELET_MEMORY, cgroup_memory: $EMPLOYED_MEMORY, cgroup_capacity_memory: $CAPACITY_MEMORY, kubepods_max_memory: $KUBEPODS_CGROUP_MEMORY_MAX } | tostring'
    )

    memory_string=$(echo $memory_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

    message_string=$( jq -n \
        --arg CGROUPV "$(echo $CGROUP_VERSION)" \
        --arg MEMORY "$(echo $memory_string)" \
        '{ CgroupVersion: $CGROUPV, Memory: $MEMORY } | tostring'
    )

else
    echo "Unexpected cgroup type. Exiting"
    exit 1
fi

message_string=$(echo $message_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

EVENT_JSON=$( jq -n \
    --arg Timestamp     "${STARTTIME_FORMATTED}" \
    --arg OperationId   "${ENDTIME_FORMATTED}" \
    --arg Version       "1.23" \
    --arg TaskName      "AKS.CSE.system_slice" \
    --arg EventLevel    "${eventlevel}" \
    --argjson Message       "${message_string}" \
    --arg EventPid      "0" \
    --arg EventTid      "0" \
    '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
)

echo ${EVENT_JSON} > ${EVENTS_LOGGING_DIR}${EVENTS_FILE_NAME}.json
cat ${EVENTS_LOGGING_DIR}${EVENTS_FILE_NAME}.json