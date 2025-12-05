For testing AKS Ubuntu GPU with FIPS to see what's in prod, do the following:

Don't create resources in eastus.

Create a resource group (usually westeurope works well) with the name gktest. via az group create -g gktest -l westeurope. The same resource group can be used to create clusters in different regions too.

Region selection: usually I want to test GPUs with FIPS, and in which case the nodepool needs to be created in a region with quota/capacity for GPU VMs that support FIPS. Check this for where GPU quota is available (though it's not exclusive and we keep getting more): https://gist.githubusercontent.com/ganeshkumarashok/52a1043d0a023b46fd4366364d7c6799/raw/3e8858e2fd400a7160bb0c9275e25a65c31d207b/gpuquotaavailable.md.

Create an AKS cluster in the correct region (very important) with FIPS enabled. Cluster name should be testcl IF that same cluster name is not being used already. Before creating though, check if the cluster/rg already exists. Command should be like: az aks create -g <rg> --name <cluster-name> --location <region> --node-count 1 --enable-fips-image

Then run az aks get-credentials -n <cluster-name> -g <rg-name> --overwrite-existing
Wait for the cluster to be created, then check with kubectl get nodes that at least 1 node is present, and if it is: then create a GPU nodepool with FIPS enabled.

If I say create a V100 GPU FIPS node, then create a 1 node nodepool with Standard_NC6s_v3 with FIPS enabled.
az aks nodepool add -g <rg> --cluster-name <cluster-name> --nodepool-name <np-name> --node-count 1 --node-vm-size Standard_NC6s_v3 --enable-fips-image. It can take 4-6 minutes for a GPU node with FIPS to be created.

If I say create an A100 FIPS node, then create a 1 node nodepool with Standard_ND96asr_v4 with FIPS enabled.
az aks nodepool add -g <rg> --cluster-name <cluster-name> --nodepool-name <np-name> --node-count 1 --node-vm-size Standard_ND96asr_v4 --enable-fips-image

If I say create an H100 ISR FIPS node, then create a 1 node nodepool with Standard_ND96isr_H100_v5 in UAE north with FIPS enabled.
az aks nodepool add -g <rg> --cluster-name <cluster-name> --nodepool-name <np-name> --node-count 1 --node-vm-size Standard_ND96isr_H100_v5 --enable-fips-image

Use kubectl get nodes to show me the status of the created nodes too --- perhaps with the --wait key.

After the GPU FIPS node is ready, verify FIPS is enabled by checking /proc/sys/crypto/fips_enabled returns 1 and that GPU drivers are working with nvidia-smi.

Use common sense and modify/change as necessary based on what I ask for.