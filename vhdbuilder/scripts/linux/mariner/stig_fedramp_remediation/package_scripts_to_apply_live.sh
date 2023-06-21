#!/bin/bash
# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

set -e

script_dir="$(dirname "$(realpath "$0" )")"
out_file="${script_dir}/remediation.tar.gz"
staging_dir="${script_dir}/stig_remediation"

mkdir -p "${staging_dir}"
mkdir -p "${staging_dir}/rhel8"

cp "${script_dir}/rhel8/"* "${staging_dir}/rhel8"
cp "${script_dir}/marketplace_compliance.sh" "${staging_dir}"
cp "${script_dir}/skip_list.txt" "${staging_dir}"
cp "${script_dir}/live_machine_only.txt" "${staging_dir}"
cp "${script_dir}/marketplace_skip_list.txt" "${staging_dir}"
cp "${script_dir}/ssg-mariner-ds.xml" "${staging_dir}"

echo "echo '    >>>> running oscap with input $(realpath ssg-mariner-ds.xml)'"  > "${staging_dir}/run_oscap.sh"
echo 'oscap xccdf eval --profile xccdf_org.ssgproject.content_profile_stig-rhel8 --stig-viewer stig-rhel8_results.xml --report stig-rhel8_report.html ssg-mariner-ds.xml 2>&1 | tee oscap.log'  >> "${staging_dir}/run_oscap.sh"
echo "echo '    >>>> oscap log: $(realpath ./ )/oscap.log'" >> "${staging_dir}/run_oscap.sh"
echo "echo '    >>>> oscap results: $(realpath ./ )/stig-rhel8_results.xml'" >> "${staging_dir}/run_oscap.sh"
echo "echo '    >>>> oscap report: $(realpath ./ )/stig-rhel8_report.html'" >> "${staging_dir}/run_oscap.sh"
echo "echo \"    >>>> consider running 'chown mariner_user:mariner_user *' before SCP'ing files over\"" >> "${staging_dir}/run_oscap.sh"
chmod +x "${staging_dir}/run_oscap.sh"

tar -czvf "${out_file}" stig_remediation

rm -r "${staging_dir}"