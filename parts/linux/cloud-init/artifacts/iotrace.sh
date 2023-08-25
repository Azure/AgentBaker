#!/usr/bin/env bash

set -xe

bpftrace -o /var/log/azure/iotrace.log -e 'tracepoint:block:block_rq_issue { @biorqcount = count(); @biorqbytes = sum(args->bytes); }  tracepoint:syscalls:sys_exit_read { @reads[comm,pid] = count(); } tracepoint:syscalls:sys_exit_write { @writes[comm,pid] = count(); } interval:s:10 { time("%H:%M:%S\n"); print(@biorqcount); zero(@biorqcount); print(@biorqbytes); zero(@biorqbytes); print(@reads); clear(@reads); print(@writes); clear(@writes); } tracepoint:syscalls:sys_enter_exec* { join(args->argv); } tracepoint:syscalls:sys_enter_open,tracepoint:syscalls:sys_enter_openat { printf("open: %s\n", str(args->filename)); }'
