#!/usr/bin/env python3

import re
from collections import defaultdict

TIME_RE = re.compile("^\d+:\d+:\d+\n$")
BIORQCOUNT_RE = re.compile("^@biorqcount: (\d+)\n$")
BIORQBYTES_RE = re.compile("^@biorqbytes: (\d+)\n$")
READ_RE = re.compile("^@reads\[([^,]+), (\d+)\]: (\d+)\n$")
WRITE_RE = re.compile("^@writes\[([^,]+), (\d+)\]: (\d+)\n$")


reports = []
with open("iotrace.log") as f:
    for line in f:
        if TIME_RE.match(line) is not None:
            reports.append({
                "endTime": line.strip(),
                "biorqcount": None,
                "biorqbytes": None,
                "reads": {},
                "writes": {},
            })
            continue

        biorqcount_match = BIORQCOUNT_RE.match(line)
        if biorqcount_match is not None:
            count = int(biorqcount_match.group(1))
            reports[-1]["biorqcount"] = count
            continue

        biorqbytes_match = BIORQBYTES_RE.match(line)
        if biorqbytes_match is not None:
            bytes = int(biorqbytes_match.group(1))
            reports[-1]["biorqbytes"] = bytes
            continue

        read_match = READ_RE.match(line)
        if read_match is not None:
            comm = read_match.group(1)
            pid = int(read_match.group(2))
            count = int(read_match.group(3))
            reports[-1]["reads"][(comm,pid)] = count
            continue

        write_match = WRITE_RE.match(line)
        if write_match is not None:
            comm = write_match.group(1)
            pid = int(write_match.group(2))
            count = int(write_match.group(3))
            reports[-1]["writes"][(comm,pid)] = count
            continue

# CSV for block IO
#for r in reports:
#    print("{},{},{}".format(r["endTime"], r["biorqcount"], r["biorqbytes"]))

# CSV for IO read/write by process in first 30 seconds
read_by_proc = defaultdict(lambda: 0)
write_by_proc = defaultdict(lambda: 0)
for r in reports[0:3]:
    for p, reads in r["reads"].items():
        read_by_proc[p[0]] += reads
    for p, writes in r["writes"].items():
        write_by_proc[p[0]] += writes


all_processes = sorted(set([x for x in read_by_proc.keys()] + [x for x in write_by_proc.keys()]))
for p in all_processes:
    total = read_by_proc[p] + write_by_proc[p]
    print("{},{},{},{}".format(p, read_by_proc[p], write_by_proc[p], total))
