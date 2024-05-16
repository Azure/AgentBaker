#!/usr/bin/expect -f

set TEST_VM_ADMIN_USERNAME "azureuser"
set TEST_VM_ADMIN_PASSWORD "TestVM@1715622512"
set FQDN "alisontestfqdn2"

set SF_TRIVY "/home/azureuser/packer/trivy-scan.sh"
set DF_TRIVY "/Users/alisonburgess/Documents/development/agentBaker/AgentBaker/vhdbuilder/packer/trivy-scan.sh"
set REMOTE_HOST "$TEST_VM_ADMIN_USERNAME@$FQDN.eastus.cloudapp.azure.com"

# upload file
spawn scp $DF_TRIVY $REMOTE_HOST:$SF_TRIVY 
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

set SF_EXE "/home/azureuser/packer/execute-vhd-scanning.sh"
set DF_EXE "/Users/alisonburgess/Documents/development/agentBaker/AgentBaker/vhdbuilder/packer/execute-vhd-scanning.sh"
spawn scp $DF_EXE $REMOTE_HOST:$SF_EXE 
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

