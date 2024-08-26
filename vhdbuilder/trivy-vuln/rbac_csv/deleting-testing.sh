#!/bin/bash

# Function to remove orphan role assignments
function Remove-Orphan-Role-Assignments {
  # Get all service principal role assignments
    roleAssignments=$(az role assignment list --query "[?principalType=='ServicePrincipal']" --all --output json)

    # Get all Azure resources that may have associated service principals (e.g., VMs, App Services, AKS, etc.)
    resources=$(az resource list --query "[].{id:id, type:type, identity:identity}" --output json)

    # Extract principalIds of all managed identities associated with Azure resources
    resourcePrincipalIds=$(echo "$resources" | jq -r '.[].identity.principalId' | grep -v null)

    orphanCount=0
    declare -a roleAssignmentIds=()
    while IFS= read -r assignment; do
        principalId=$(echo "$assignment" | jq -r '.principalId')
        roleAssignmentId=$(echo "$assignment" | jq -r '.id')
        roleName=$(echo "$assignment" | jq -r '.roleDefinitionName')

        # Try to process role assignment
        if ! echo "$vmPrincipalIds" | grep -q "$principalId"; then
            if [[ "$roleAssignmentId" == *"/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/"* ]]; then
                if [[ "$roleName" == "Storage Blob Data Contributor" ]]; then
                    orphanCount=$((orphanCount + 1))
                    echo "Deleting: $roleName, $roleAssignmentId, $principalId"
                    roleAssignmentIds+=("$roleAssignmentId")
                fi 
            fi
        fi
    done < <(echo "$roleAssignments" | jq -c '.[]' || echo "[]")

    if [[ ${#roleAssignmentIds[@]} -gt 0 ]]; then
        echo "Deleting $orphanCount orphaned role assignments..."

        # Join role assignment IDs into a space-separated string
        idsToDelete=$(IFS=" "; echo "${roleAssignmentIds[*]}")

        echo "About to begin batch deletion" 

        # Perform batch deletion
        # az role assignment delete --ids $idsToDelete
    else
        echo "No orphaned role assignments found to delete."
    fi

    echo "Orphans removed: $orphanCount"
}

Remove-Orphan-Role-Assignments