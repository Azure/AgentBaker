#!/bin/bash
# AIB entry point that keeps its historical path: vhdbuilder/prefetch/templates/optimize.json and the
# aks-rp hotfix prefetch template run this script after they customize a VHD built by Packer.
# AIB runs /tmp/DeprovisioningScript.sh in place of its default waagent deprovision as the last step,
# so generalization runs after all customizations. aks-node-controller generalize removes the hook.
set -euo pipefail

printf '%s\n' '#!/bin/bash -eu' 'exec sudo /opt/azure/containers/aks-node-controller generalize' > /tmp/DeprovisioningScript.sh
chmod 0755 /tmp/DeprovisioningScript.sh
