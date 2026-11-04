#!/bin/bash

Describe 'init-aks-cloud.sh prerequisite certificate phase'
    script_path='./parts/linux/cloud-init/artifacts/init-aks-cloud.sh'

    setup() {
        LOCATION=westus2
        REPO_DEPOT_ENDPOINT=''
        IS_UBUNTU=1
        IS_MARINER=0
        IS_AZURELINUX=0
        export OPT_IN_RESULT=0 CERT_RETRIEVE_EXIT=0 CERT_INSTALL_EXIT=0
        unset AKS_CLOUD_PREREQUISITES_DONE
    }
    BeforeEach setup

    run_phase() {
        # Execute the real dispatch with OS setup omitted and host mutations mocked.
        . <(sed -n '/^refresh_location=/,$p' "$script_path") "$@"
    }

    Mock mkdir
        exit 0
    End
    Mock rm
        exit 0
    End
    Mock emit_event
        exit 0
    End
    Mock logs_to_events
        shift
        "$@"
    End
    Mock determine_cert_endpoint_mode
        case "$1" in
            ussec*|usnat*) echo legacy ;;
            *) echo rcv1p ;;
        esac
    End
    Mock is_opted_in_for_root_certs
        echo 'checked opt-in'
        exit "$OPT_IN_RESULT"
    End
    Mock retrieve_legacy_certs
        echo 'retrieved legacy certificates'
        exit "$CERT_RETRIEVE_EXIT"
    End
    Mock retrieve_rcv1p_certs
        echo 'retrieved rcv1p certificates'
        exit "$CERT_RETRIEVE_EXIT"
    End
    Mock install_certs_to_trust_store
        echo 'installed certificates'
        exit "$CERT_INSTALL_EXIT"
    End
    Mock find
        echo '/root/AzureCACertificates/cloud.crt'
    End
    Mock crontab
        if [ "$1" = "-" ]; then
            cat >/dev/null
            echo 'scheduled refresh'
        fi
    End
    Mock aptget_update
        echo 'refreshed indexes'
    End
    Mock lsb_release
        case "$1" in
            --release) printf 'Release:\t22.04\n' ;;
            -c) printf 'Codename:\tjammy\n' ;;
        esac
    End
    Mock init_ubuntu_main_repo_depot
        echo "ubuntu-main $1"
    End
    Mock init_ubuntu_pmc_repo_depot
        echo "ubuntu-pmc $1"
    End
    Mock init_mariner_repo_depot
        echo "mariner $1"
    End
    Mock init_azurelinux_repo_depot
        echo "azurelinux $1"
    End
    Mock dnf_makecache
        echo 'refreshed RPM indexes'
    End

    Describe 'legacy no-argument invocation'
        Parameters
            1 0 0 ubuntu-main 'refreshed indexes' ussec
            1 0 0 ubuntu-main 'refreshed indexes' westus2
            0 1 0 mariner 'refreshed RPM indexes' ussec
            0 0 1 azurelinux 'refreshed RPM indexes' westus2
        End

        It 'runs certificates, repositories, indexes, and scheduling without a launcher handoff'
            IS_UBUNTU=$1
            IS_MARINER=$2
            IS_AZURELINUX=$3
            LOCATION=$6
            REPO_DEPOT_ENDPOINT=https://depot.example/ubuntu

            When run run_phase
            The status should be success
            The output should include 'installed certificates'
            The output should include "$4 https://depot.example"
            The output should include "$5"
            The output should include 'scheduled refresh'
        End
    End

    Describe 'successful bootstrap'
        Parameters
            ussec legacy
            westus2 rcv1p
        End
        It 'installs cloud certificates without scheduling or refreshing indexes'
            LOCATION=$1
            When run run_phase prerequisites
            The status should be success
            The output should include "retrieved $2 certificates"
            The output should include 'installed certificates'
            The output should not include 'scheduled refresh'
            The output should not include 'refreshed indexes'
        End
    End

    It 'preserves public-cloud opt-out without installing certificates'
        export OPT_IN_RESULT=1
        When run run_phase prerequisites
        The status should be success
        The output should include 'checked opt-in'
        The output should not include 'retrieved'
        The output should not include 'installed certificates'
    End

    It 'fails when certificate opt-in cannot be determined'
        export OPT_IN_RESULT=2
        When run run_phase prerequisites
        The status should eq 1
        The output should include 'wireserver unreachable'
        The output should not include 'installed certificates'
    End

    Describe 'required bootstrap failure'
        Parameters
            ussec 18 0
            ussec 0 19
            westus2 18 0
            westus2 0 19
        End
        It 'fails for certificate retrieval or trust-store installation errors'
            LOCATION=$1
            export CERT_RETRIEVE_EXIT=$2 CERT_INSTALL_EXIT=$3
            When run run_phase prerequisites
            The status should eq 1
            The output should include 'retrieved'
            if [ "$3" -ne 0 ]; then
                The stderr should include 'failed to install'
            else
                The output should include 'failed to retrieve'
            fi
            The output should not include 'scheduled refresh'
        End
    End

    It 'uses the successful process handoff without repeating certificate bootstrap'
        AKS_CLOUD_PREREQUISITES_DONE=true
        When run run_phase init
        The status should be success
        The output should include 'scheduled refresh'
        The output should not include 'checked opt-in'
        The output should not include 'retrieved'
        The output should not include 'installed certificates'
    End

    It 'ignores the handoff on a new prerequisite invocation'
        AKS_CLOUD_PREREQUISITES_DONE=true
        When run run_phase prerequisites
        The status should be success
        The output should include 'checked opt-in'
        The output should include 'installed certificates'
    End

    It 'continues retrieving fresh certificates during scheduled refresh'
        AKS_CLOUD_PREREQUISITES_DONE=true
        When run run_phase ca-refresh westus2
        The status should be success
        The output should include 'checked opt-in'
        The output should include 'installed certificates'
        The output should not include 'scheduled refresh'
    End
End
