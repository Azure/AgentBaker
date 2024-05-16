#!/usr/bin/expect -f

set TEST_VM_ADMIN_USERNAME "azureuser"
set TEST_VM_ADMIN_PASSWORD "TestVM@1715622512"
set FQDN "alisontestfqdn2"

set SF_TRIVY "/home/azureuser/packer/trivy-scan.sh"
set DF_TRIVY "/Users/alisonburgess/Documents/development/agentBaker/AgentBaker/vhdbuilder/packer/trivy-scan.sh"
set REMOTE_HOST "$TEST_VM_ADMIN_USERNAME@$FQDN.eastus.cloudapp.azure.com"

set SF_TRIVY_REPORT "/opt/azure/containers/trivy-report.json"
set SF_TRIVY_TABLE "/opt/azure/containers/trivy-images-table.txt"

# trivy report
spawn scp $REMOTE_HOST:$SF_TRIVY_REPORT .
expect {
    -re {Are you sure you want to continue connecting \(yes/no/\[fingerprint\]\)\?} {
        send "yes\r"
        exp_continue
    }
    "password:" {
        send "$TEST_VM_ADMIN_PASSWORD\r"
        exp_continue
    }
    eof
}

spawn scp $REMOTE_HOST:$SF_TRIVY_TABLE .
expect {
    -re {Are you sure you want to continue connecting \(yes/no/\[fingerprint\]\)\?} {
        send "yes\r"
        exp_continue
    }
    "password:" {
        send "$TEST_VM_ADMIN_PASSWORD\r"
        exp_continue
    }
    eof
}