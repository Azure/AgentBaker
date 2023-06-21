#!/bin/bash
# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

while (( "$#" )); do
    case "$1" in
        -s|--skip_apply)
            skip_apply="yes"
            shift
            ;;
        -l|--run_live)
            run_live="yes"
            shift
            ;;
        -m|--marketplace)
            marketplace="yes"
            shift
            ;;

        *)
            echo "Invalid arg '$1'"
            exit
            ;;
    esac
done


script_dir="$(dirname "$(realpath "$0")")"

mkdir "$script_dir/apply_logs"
echo "Apply scripts" 1>&2
touch "$script_dir/fail.txt"
touch "$script_dir/success.txt"
touch $script_dir/failure_details.txt


for script in $(find "$script_dir/rhel8" -name '*.sh' | sort -u); do
    scriptname="$(basename "${script}")"
    prunedname=$(echo "${scriptname#*-}" | cut -d'.' -f1)



    echo "checking '${prunedname}'"  1>&2
    if grep -q -E "^${prunedname}\$" "$script_dir/skip_list.txt" ; then
        # If we are running live scripts, run those anyways
        if [[ "${run_live}" == "yes" ]]; then
            if ! grep -q -E "^${prunedname}\$" "$script_dir/live_machine_only.txt" ; then
                echo "Skipping ${script} since its in skip_list.txt but not also in live_machine_only.txt" 1>&2
                continue
            fi
        else
            echo "Skipping ${script} since its in skip_list.txt" 1>&2
            continue
        fi
    fi

    if [[ "${marketplace}" == "yes" ]]; then
    if grep -q -E "^${prunedname}\$" "$script_dir/marketplace_skip_list.txt" ; then
        echo "Skipping ${script} since its in marketplace_skip_list.txt" 1>&2
        continue
    fi
    fi

    if [[ "${skip_apply}" == "yes" ]]; then
        echo "Skipping ${script} due to --skip_apply"  1>&2
        echo "Skipped ${script}" > "$script_dir/apply_logs/$(basename "${script}").log"
    else
        echo "Running ${script}" 1>&2
        out=$(${script} 2>&1)
        res=$?
        if [[ ${res} -ne 0 ]]; then
            basename "${script}" >> "$script_dir/fail.txt"
        else
            basename "${script}" >> "$script_dir/success.txt"
        fi
        echo "$out" > "$script_dir/apply_logs/$(basename "${script}").log"
    fi
done


if [[ $(wc -l < "$script_dir/fail.txt") -gt 0 ]]; then
    cat "$script_dir/fail.txt"
    while read -r line; do
        echo "${line}:" | tee -a $script_dir/failure_details.txt
        cat "$script_dir/apply_logs/${line}.log" | tee -a $script_dir/failure_details.txt
    done < "$script_dir/fail.txt"
    exit 1
fi
