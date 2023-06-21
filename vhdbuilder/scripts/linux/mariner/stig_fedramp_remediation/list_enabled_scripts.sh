#!/bin/bash
# Lists every script which is in the rhel8 folder, but not listed in the skip list or the status file.
# ie lists all remediation scripts we will run during build

# Also checks for profile files in ../ComplianceAsCode and generates lists for them.

set -e

list_dir=./lists
mkdir -p ${list_dir}
files="${list_dir}/enabled_scripts.txt ${list_dir}/all_scripts.txt ${list_dir}/skipped_scripts.txt ${list_dir}/live_scripts.txt ${list_dir}/missing_fix.txt"
for f in $files; do
    rm -f ./$f
    touch ./$f
done

for script in $(find ./rhel8/ -name "*.sh" | sort -u); do
    scriptname="$(basename ${script})"
    prunedname=$(echo ${scriptname#*-} | cut -d'.' -f1)

    if [[ -z "$(grep "FIX FOR THIS RULE '.*' IS MISSING" ./${script})" ]]; then
        echo ${prunedname} >> ${list_dir}/all_scripts.txt
    else
        echo "Skipping ${scriptname} since it has no fix"
        line="${prunedname}"

        cacrule=$(find ../../ComplianceAsCode -path **/${prunedname}/** -name rule.yml)
        if [[ -n "$(grep "template:" ${cacrule})" ]]; then
            template="$(sed -n '/template:/,$p' ${cacrule} | grep "^[[:space:]]*name:" | awk '{print $2}')"

            if [[ -f "../../ComplianceAsCode/shared/templates/${template}/bash.template" ]]; then
                line="${line},TEMPLATE"
            else
                line="${line},NO"
            fi
        else
            scripts=$(find $(dirname ${cacrule})/bash -name '*.sh' 2>/dev/null || true)
            if [[ -n "${scripts}" ]]; then
                line="${line},YES"
            else
                line="${line},NO"
            fi
        fi
        echo ${line} >> ${list_dir}/missing_fix.txt
        continue
    fi

    echo "checking ${prunedname} from ${script}"
    enable="true"
    if [[ -n "$(grep -E "^${prunedname}\$" ./skip_list.txt )" ]]; then
        echo "Skipping ${scriptname} since its in skip_list.txt"
        echo "${prunedname}" >> ${list_dir}/skipped_scripts.txt
        enable="false"
    fi

    if [[ -n "$(grep -E "^${prunedname}\$" ./live_machine_only.txt )" ]]; then
        echo "Skipping ${scriptname} since its in live_machine_only.txt"
        echo "${prunedname}" >> ${list_dir}/live_scripts.txt
        enable="false"
    fi

    if [[ $enable != "false" ]]; then
        echo "${prunedname}" >> ${list_dir}/enabled_scripts.txt
    fi
done

for f in $files; do
    sort -o ./$f ./$f
done

profile_list="../../ComplianceAsCode/products/mariner/profiles/stig.profile ../../ComplianceAsCode/products/mariner/profiles/stig-rhel8.profile"
for profile in $profile_list; do
    if [[ -f $profile ]]; then
        echo "Getting data from $profile"
        cat $profile \
            | sed '1,/^selections:$/d' \
            | grep --invert-match "^ *#.*$" \
            | grep --invert-match "^ *$" \
            | grep --invert-match ".*=.*"  \
            | rev | cut -d' ' -f1 | rev \
            | sort -u > ${list_dir}/$(basename $profile).txt
    else
        echo "CAN'T FIND $profile"
    fi
done