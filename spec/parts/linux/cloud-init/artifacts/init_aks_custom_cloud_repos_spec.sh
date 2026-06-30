#!/bin/bash

Describe 'init-aks-custom-cloud-repos.sh repo depot + chrony wiring'
    script_path='./parts/linux/cloud-init/artifacts/init-aks-custom-cloud-repos.sh'

    Describe 'function definitions'
        Parameters
            init_ubuntu_main_repo_depot
            init_ubuntu_pmc_repo_depot
            init_mariner_repo_depot
            init_azurelinux_repo_depot
            check_url
            write_to_sources_file
            add_key_ubuntu
            add_ms_keys
            derive_key_paths
            aptget_update
            dnf_makecache
        End

        It "defines function $1"
            When run grep -Eq "^function $1 \\{$" "$script_path"
            The status should eq 0
        End
    End

    Describe 'distro branching ladder'
        It 'routes Ubuntu to Ubuntu repo init branch'
            When run grep -Eq '^if \[ "\$IS_UBUNTU" -eq 1 \]; then$' "$script_path"
            The status should eq 0
        End

        It 'routes Mariner or AzureLinux to the rpm-based repo init branch'
            When run grep -Eq '^elif \[ "\$IS_MARINER" -eq 1 \] \|\| \[ "\$IS_AZURELINUX" -eq 1 \]; then$' "$script_path"
            The status should eq 0
        End

        It 'invokes Mariner-specific init only when IS_MARINER=1'
            When run grep -Eq '^[[:space:]]*if \[ "\$IS_MARINER" -eq 1 \]; then$' "$script_path"
            The status should eq 0
        End

        It 'falls back to AzureLinux init in the else branch'
            When run grep -Eq 'init_azurelinux_repo_depot \$\{marinerRepoDepotEndpoint\}' "$script_path"
            The status should eq 0
        End
    End

    Describe 'Ubuntu apt sources rewrite'
        It 'syncs OpenSSL bundle from system trust store'
            When run grep -Fq 'cp /etc/ssl/certs/ca-certificates.crt /usr/lib/ssl/cert.pem' "$script_path"
            The status should eq 0
        End

        It 'backs up existing /etc/apt/sources.list before rewrite'
            When run grep -Fq 'mv /etc/apt/sources.list /etc/apt/backup/' "$script_path"
            The status should eq 0
        End

        It 'writes the new ubuntu.sources file under sources.list.d'
            When run grep -Fq '/etc/apt/sources.list.d/ubuntu.sources' "$script_path"
            The status should eq 0
        End

        It 'rewrites all http(s) URLs in apt sources to the RepoDepot URL'
            When run grep -Fq 'sed -i "s,https\?://.[^ ]*,$ubuntuUrl,g" $aptSourceFile' "$script_path"
            The status should eq 0
        End
    End

    Describe 'parameter passing — add_key_ubuntu / add_ms_keys take repodepot_endpoint explicitly'
        # Guards against regression of fix(ef6...) where these were sourced from
        # an outer-scope variable instead of being passed as an argument.
        It 'add_key_ubuntu declares repodepot_endpoint as first positional arg'
            When run grep -Eq '^    local repodepot_endpoint="\$1"$' "$script_path"
            The status should eq 0
        End

        It 'add_ms_keys forwards repodepot_endpoint to add_key_ubuntu'
            When run grep -Fq 'add_key_ubuntu "$repodepot_endpoint" microsoft.asc' "$script_path"
            The status should eq 0
        End
    End

    Describe 'Mariner rpm repo creation'
        It 'creates mariner-extended.repo from mariner-extras.repo'
            When run grep -Fq 'cp /etc/yum.repos.d/mariner-extras.repo /etc/yum.repos.d/mariner-extended.repo' "$script_path"
            The status should eq 0
        End

        It 'creates mariner-nvidia.repo from mariner-extras.repo'
            When run grep -Fq 'cp /etc/yum.repos.d/mariner-extras.repo /etc/yum.repos.d/mariner-nvidia.repo' "$script_path"
            The status should eq 0
        End

        It 'creates mariner-cloud-native.repo from mariner-extras.repo'
            When run grep -Fq 'cp /etc/yum.repos.d/mariner-extras.repo /etc/yum.repos.d/mariner-cloud-native.repo' "$script_path"
            The status should eq 0
        End

        It 'redirects packages.microsoft.com URLs to RepoDepot for all .repo files'
            When run grep -Eq 'sed -i -e "s\|https://packages\.microsoft\.com\|\$\{repodepot_endpoint\}/mariner/packages\.microsoft\.com\|"' "$script_path"
            The status should eq 0
        End
    End

    Describe 'AzureLinux tdnf repo creation'
        It 'removes pre-existing azurelinux*.repo before writing fresh repos'
            When run grep -Fq 'rm -f /etc/yum.repos.d/azurelinux*' "$script_path"
            The status should eq 0
        End

        It 'enumerates the full set of AzureLinux repos to create'
            When run grep -Fq 'local repos=("amd" "base" "cloud-native" "extended" "ms-non-oss" "ms-oss" "nvidia")' "$script_path"
            The status should eq 0
        End

        It 'enables gpgcheck on generated repos'
            When run grep -Fq '"gpgcheck=1"' "$script_path"
            The status should eq 0
        End

        It 'enables repo_gpgcheck on generated repos'
            When run grep -Fq '"repo_gpgcheck=1"' "$script_path"
            The status should eq 0
        End

        It 'pins gpgkey to MICROSOFT-RPM-GPG-KEY'
            When run grep -Fq '"gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY"' "$script_path"
            The status should eq 0
        End
    End

    Describe 'chrony configuration branching'
        It 'skips chrony configuration entirely on ACL'
            When run grep -Eq '^if \[ "\$IS_ACL" -eq 1 \]; then$' "$script_path"
            The status should eq 0
        End

        It 'documents the ACL skip with a logged reason'
            When run grep -Fq 'Skipping chrony configuration for ACL' "$script_path"
            The status should eq 0
        End

        It 'writes /etc/chrony.conf for Mariner/AzureLinux'
            When run grep -Eq '^elif \[ "\$IS_MARINER" -eq 1 \] \|\| \[ "\$IS_AZURELINUX" -eq 1 \]; then$' "$script_path"
            The status should eq 0
        End

        It 'configures PTP refclock for Mariner/AzureLinux chrony'
            When run grep -Fq 'refclock PHC /dev/ptp0 poll 3 dpoll -2 offset 0' "$script_path"
            The status should eq 0
        End

        It 'restarts chronyd on Mariner/AzureLinux branch'
            When run grep -Eq '^systemctl restart chronyd$' "$script_path"
            The status should eq 0
        End

        It 'targets /etc/chrony/chrony.conf for Ubuntu/Flatcar'
            When run grep -Eq '^chrony_conf="/etc/chrony/chrony\.conf"$' "$script_path"
            The status should eq 0
        End

        It 'disables systemd-timesyncd on Ubuntu before installing chrony'
            When run grep -Eq '^[[:space:]]*systemctl stop systemd-timesyncd$' "$script_path"
            The status should eq 0
        End

        It 'installs chrony on Ubuntu when not already present'
            When run grep -Eq '^[[:space:]]*apt-get install chrony -y$' "$script_path"
            The status should eq 0
        End

        It 'removes the default chrony config on Flatcar to force regeneration'
            When run grep -Eq '^[[:space:]]*rm -f \$\{chrony_conf\}$' "$script_path"
            The status should eq 0
        End
    End

    Describe 'chrony restart telemetry (regression guard for commit f1233050ba)'
        It 'wraps Ubuntu chrony restart in logs_to_events'
            When run grep -Fq 'logs_to_events "AKS.CSE.customCloud.restartChrony" systemctl restart chrony' "$script_path"
            The status should eq 0
        End

        It 'wraps Flatcar chronyd restart in logs_to_events'
            When run grep -Fq 'logs_to_events "AKS.CSE.customCloud.restartChrony" systemctl restart chronyd' "$script_path"
            The status should eq 0
        End
    End

    Describe 'sourcing contract — script must be safe to source'
        It 'does not call exit at top level (would terminate the sourcing parent)'
            # Exits only allowed inside functions (check_url, aptget_update, dnf_makecache)
            # or guarded by branch-internal error handling. No bare top-level exit.
            When run grep -En '^exit( |$)' "$script_path"
            The status should eq 1
        End

        It 'enables shell tracing for diagnostic logs'
            When run grep -Eq '^set -x$' "$script_path"
            The status should eq 0
        End
    End
End
