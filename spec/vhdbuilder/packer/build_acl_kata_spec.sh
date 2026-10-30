#!/usr/bin/env shellspec

Describe 'build-acl-kata.sh'
  setup() {
    TEST_ROOT="${SHELLSPEC_TMPBASE}/build-acl-kata"
    MOCK_BIN="${TEST_ROOT}/bin"
    BASE_TEMPLATE="${TEST_ROOT}/acl.json"
    CAPTURED_TEMPLATE="${TEST_ROOT}/acl-kata.json"
    rm -rf "$TEST_ROOT"
    mkdir -p "$MOCK_BIN"
    cat > "$BASE_TEMPLATE" <<'EOF'
{"variables":{"feature_flags":"{{env `FEATURE_FLAGS`}}"},"builders":[{"type":"azure-arm"}],"provisioners":[{"type":"file","source":"a","destination":"/home/packer/a"},{"type":"shell","inline":["sudo /bin/bash -eux /home/packer/pre-install-dependencies.sh"]},{"type":"shell","inline":": reboot"},{"type":"shell","inline":["sudo /bin/bash -eux /home/packer/install-dependencies.sh"]},{"type":"shell","inline":["sudo /bin/bash -eux /home/packer/post-install-dependencies.sh"]}]}
EOF
    cat > "${MOCK_BIN}/packer" <<'EOF'
#!/bin/sh
for argument in "$@"; do
  template=$argument
done
cp "$template" "$CAPTURED_TEMPLATE"
EOF
    chmod +x "${MOCK_BIN}/packer"
    export CAPTURED_TEMPLATE
    PATH="${MOCK_BIN}:${PATH}"
    export PATH
  }

  BeforeEach 'setup'

  # The Kata steps are added around the build scripts and nothing else changes.
  build_and_validate_kata_template() {
    ./vhdbuilder/packer/build-acl-kata.sh &&
      jq -e --slurpfile base "$ACL_PACKER_TEMPLATE" '
        def runs($script): (.inline // "") | tostring | contains("/bin/bash -eux /home/packer/" + $script);
        ([.provisioners[] | runs("pre-install-dependencies.sh")] | index(true)) as $pre |
        ([.provisioners[] | runs("install-dependencies.sh")] | index(true)) as $install |
        .provisioners[$pre - 2:$pre] == [
          {"type": "file", "source": "kata-containers.img", "destination": "/home/packer/kata-containers.img"},
          {"type": "file", "content": "{{user `kata_uvm_version`}}\n", "destination": "/home/packer/kata-containers-version.txt"}
        ] and
        .provisioners[$install + 1] == {
          "type": "shell",
          "script": "vhdbuilder/packer/validate-kata-uvm-version.sh",
          "environment_vars": [
            "FEATURE_FLAGS={{user `feature_flags`}}",
            "OS_SKU=AzureContainerLinux",
            "SYSTEM_COLLECTIONURI={{user `system_collection_uri`}}",
            "SYSTEM_TEAMPROJECTID={{user `system_team_project_id`}}"
          ]
        } and
        .variables.kata_uvm_version == "{{env `KATA_UVM_VERSION`}}" and
        .variables.system_collection_uri == "{{env `SYSTEM_COLLECTIONURI`}}" and
        .variables.system_team_project_id == "{{env `SYSTEM_TEAMPROJECTID`}}" and
        (.provisioners | del(.[$install + 1]) | del(.[$pre - 2:$pre])) == $base[0].provisioners and
        (.variables | del(.kata_uvm_version, .system_collection_uri, .system_team_project_id)) == $base[0].variables and
        .builders == $base[0].builders
      ' "$CAPTURED_TEMPLATE" >/dev/null
  }

  It 'adds the guest image uploads and the version check around the build scripts'
    ACL_PACKER_TEMPLATE="$BASE_TEMPLATE"
    export ACL_PACKER_TEMPLATE
    When call build_and_validate_kata_template
    The status should be success
    The output should include "Using ACL Kata image settings derived from $BASE_TEMPLATE"
  End

  It 'derives from the production ACL template'
    ACL_PACKER_TEMPLATE=vhdbuilder/packer/vhd-image-builder-acl.json
    export ACL_PACKER_TEMPLATE
    When call build_and_validate_kata_template
    The status should be success
    The output should include "Using ACL Kata image settings derived from $ACL_PACKER_TEMPLATE"
  End

  It 'fails when the base template does not run install-dependencies.sh'
    jq 'del(.provisioners[3])' "$BASE_TEMPLATE" > "${TEST_ROOT}/acl-without-install.json"
    ACL_PACKER_TEMPLATE="${TEST_ROOT}/acl-without-install.json"
    export ACL_PACKER_TEMPLATE
    When run bash ./vhdbuilder/packer/build-acl-kata.sh
    The status should be failure
    The stderr should include 'no provisioner runs install-dependencies.sh'
  End
End
