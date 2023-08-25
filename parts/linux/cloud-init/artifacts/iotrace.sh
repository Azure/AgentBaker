#!/usr/bin/env bash

set -xe

bpftrace -o /var/log/azure/iotrace.log -e 'tracepoint:block:block_rq_issue { @biorqcount[comm,pid] = count(); @biorqbytes[comm,pid] = sum(args->bytes); }  tracepoint:syscalls:sys_exit_read { @reads[comm,pid] = count(); } tracepoint:syscalls:sys_exit_write { @writes[comm,pid] = count(); } interval:s:10 { printf("----\n"); time("%H:%M:%S\n"); print(@biorqcount); zero(@biorqcount); print(@biorqbytes); zero(@biorqbytes); print(@reads); clear(@reads); print(@writes); clear(@writes); } tracepoint:syscalls:sys_enter_exec* { join(args->argv); }'
