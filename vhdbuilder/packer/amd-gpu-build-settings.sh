#!/bin/bash

# Loaded only by an explicit AMD build. Shared CPU/NVIDIA settings stay in their
# existing path and do not depend on this image's configuration or validation.
function validate_amd_gpu_build() {
	if [ "${FEATURE_FLAGS:-}" != "AMD_GPU" ] || [ "${OS_SKU:-}" != "Ubuntu" ] ||
		[ "${OS_VERSION:-}" != "24.04" ] || [ "${ARCHITECTURE,,}" != "x86_64" ] ||
		[ "${HYPERV_GENERATION,,}" != "v2" ] || [ "${ENABLE_FIPS,,}" = "true" ] ||
		[ "${ENABLE_TRUSTED_LAUNCH,,}" = "true" ] || [ "${TRUSTED_LAUNCH_SUPPORTED,,}" = "true" ]; then
		echo "AMD_GPU requires Ubuntu 24.04 x86_64 Gen2, without other feature flags, FIPS or Trusted Launch" >&2
		return 1
	fi
	if [ -n "${SKU_NAME:-}" ] && [ "${SKU_NAME}" != "2404gen2amdgpucontainerd" ]; then
		echo "AMD_GPU requires the dedicated SKU_NAME 2404gen2amdgpucontainerd" >&2
		return 1
	fi
	if [ -n "${SIG_IMAGE_NAME:-}" ] && [ "${SIG_IMAGE_NAME}" != "2404gen2amdgpucontainerd" ]; then
		echo "AMD_GPU requires the dedicated SIG_IMAGE_NAME 2404gen2amdgpucontainerd" >&2
		return 1
	fi
}

function get_amd_gpu_sku_name() {
	validate_amd_gpu_build || return 1
	printf '%s\n' '2404gen2amdgpucontainerd'
}

# Only the manual AMD pipeline calls this function. The shared checked-in Packer
# template stays unchanged, and ordinary image builds never upload AMD files.
function prepare_amd_gpu_packer_template() {
	local template="$1" mappings="$2" temporary status
	validate_amd_gpu_build || return 1
	temporary=$(mktemp "${template}.amd.XXXXXX") || return 1
	jq --slurpfile mappings "${mappings}" '
      def install_boundary:
        .type == "shell" and any(.inline[]?;
          test("(^|[[:space:]])/home/packer/install-dependencies\\.sh([[:space:]]|$)"));
      $mappings[0] as $uploads |
      if ($mappings | length) != 1 or ($uploads | type) != "array" or
         ($uploads | length) == 0 or
         (all($uploads[]; .type == "file" and (.source | type) == "string" and
           (.destination | type) == "string") | not) or
         ([$uploads[].destination] | length) != ([$uploads[].destination] | unique | length)
      then error("invalid AMD Packer file mappings")
      elif any(.provisioners[]; . as $existing |
        any($uploads[]; .destination == $existing.destination and . != $existing))
      then error("conflicting AMD Packer file destination")
      else .provisioners |= (
        map(. as $existing | select(all($uploads[]; . != $existing))) |
        [to_entries[] | select(.value | install_boundary) | .key] as $boundaries |
        if ($boundaries | length) != 1
        then error("expected exactly one install-dependencies.sh provisioner")
        else .[:$boundaries[0]] + $uploads + .[$boundaries[0]:]
        end)
      end
    ' "${template}" >"${temporary}" && mv "${temporary}" "${template}"
	status=$?
	rm -f "${temporary}"
	return "${status}"
}
