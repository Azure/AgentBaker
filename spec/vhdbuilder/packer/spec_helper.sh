#shellcheck shell=sh

# Spec helper for vhdbuilder/packer tests

spec_helper_configure() {
  # Set default values for variables that might not be set
  export MODE=${MODE:-""}
  export OS_SKU=${OS_SKU:-""}
  export OS_VERSION=${OS_VERSION:-""}
  export ENABLE_FIPS=${ENABLE_FIPS:-""}
  export UA_TOKEN=${UA_TOKEN:-""}
}