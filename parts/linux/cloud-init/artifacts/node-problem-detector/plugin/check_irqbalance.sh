#!/bin/bash

OK=0
NOTOK=1

mlx_interrupts="$(cat /proc/interrupts | grep mlx | awk '{print $1}' | tr -d ':')"
mlx_interrupt_count="$(echo $mlx_interrupts | tr ' ' '\n' | wc -l)"
smp_affinities="$(for i in ${mlx_interrupts} ; do [ -f "/proc/irq/${i}/smp_affinity" ] && cat "/proc/irq/${i}/smp_affinity"; done | sort | uniq | wc -l)"

if [ "${smp_affinities}" -eq "1" ]; then
    echo "$mlx_interrupt_count IRQs with affinity to $smp_affinities CPUs"
    exit $NOTOK
fi

exit $OK
