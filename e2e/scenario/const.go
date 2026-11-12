package scenario

const (
	buildIDTagKey    = "buildID"
	defaultNamespace = "default"
)

// The e2e cluster is deliberately created on a NON-default service CIDR, so that
// anything which silently assumes the 10.0.0.0/16 default is caught here rather
// than in production. These must stay in lockstep: clusterDNSServiceIP is the
// .10 address of clusterServiceCIDR, and it is where kube-dns is allocated.
const (
	clusterServiceCIDR  = "172.16.0.0/16"
	clusterDNSServiceIP = "172.16.0.10"
)

// The LocalDNS pod ("cluster") listener. A LocalDNS-enabled pool has kubelet
// pointed here, so this is the nameserver written into every pod's resolv.conf
// at sandbox creation -- and the address the pod-DNS fallback must keep serving
// when localdns itself is down.
const localDNSClusterListenerIP = "169.254.10.11"

// cse output parsing consts
const (
	extensionErrorCodeRegex   = `ProvisioningState/failed/(\d+)`
	linuxExtensionExitCodeStr = `Enable failed: failed to execute command: command terminated with exit status=(\d+)`
)

// cseExtensionName is the name given to the CustomScript (CSE) extension when it is attached to
// the test VMSS (see createVMSSModel). It is used to locate the CSE status in a VM instance view
// deterministically, so exit-code classification stays correct even as scenarios attach other
// extensions.
const cseExtensionName = "vmssCSE"

// cseExitCodeOutboundConnFail is the CSE exit code for ERR_OUTBOUND_CONN_FAIL
// (see parts/linux/cloud-init/artifacts/cse_helpers.sh). It indicates the node's
// outbound connectivity preflight check (curl to mcr.microsoft.com, optionally routed
// through the e2e HTTP proxy) failed all retries and the script exited before kubelet
// started. In the e2e environment this is a known low-rate transient infrastructure
// flake rather than a product regression, so the harness retries node provisioning a
// bounded number of times to reduce PR-gate noise. A genuine regression fails on every
// attempt and still surfaces after the retry budget is exhausted.
const cseExitCodeOutboundConnFail = "50"

// test data used across multiple test cases
const (
	// Self-signed CA, valid through October 2036; both PEM and base64 are below 35 KB.
	encodedTestCert = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURNekNDQWh1Z0F3SUJBZ0lVUVNmNWUvTzl2MWpS" +
		"UnFQa2p2NmR5OEhydnZRd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0lURWZNQjBHQTFVRUF3d1dRV2RsYm5S" +
		"Q1lXdGxjaUJGTWtVZ1ZHVnpkQ0JEUVRBZUZ3MHlOakV4TURNeQpNalU0TXpGYUZ3MHpOakV3TXpFeU1q" +
		"VTRNekZhTUNFeEh6QWRCZ05WQkFNTUZrRm5aVzUwUW1GclpYSWdSVEpGCklGUmxjM1FnUTBFd2dnRWlN" +
		"QTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUN1cnR4M2gzTjAKMDQxQTJpWE5Ud2RR" +
		"WkhCaVpJYkxoZjVjQlB4MGZYZHlHakcvOE9aT3VBeCtOWC9ZNEk5THl5YTVaWDE3ckZFZApialNtTGFw" +
		"ckx5N3BFQ0RXcUhpNHpPSlRvOXdZWmU2a21ZZ1hGbk9zcXVGc1Blalc1SFZMV3BnTENGeGVNSkE5CmZC" +
		"VTQzRThuM1FnR0F1OFVDMlJMS1FWb0tCUkM2alk5cEpidU9uZ2NqeUhZRFh0b3M1VUFLK2dQM1JHNy9O" +
		"LzIKSENLQkVlaHE0bGVsb0pURkFNdDdBWWtvWDVRak9qSDdOcXdVQWhPemtkKzJESGZTWkQ2M05RN0c1" +
		"WDFuUTRFQwpGK2dQc2hLenhiNHQwaGUvUWFtdCt6LzUxKzY0RmNxM2ZCWWlia3c0bTc3NTI0ZEhvdzYx" +
		"aHhRZkNsVGdtejNyCldEcThYajJJaXhMWEFnTUJBQUdqWXpCaE1CMEdBMVVkRGdRV0JCUTY1WTNHNXlZ" +
		"eENlZWFYYlpWT0t1V1NrekQKQmpBZkJnTlZIU01FR0RBV2dCUTY1WTNHNXlZeENlZWFYYlpWT0t1V1Nr" +
		"ekRCakFQQmdOVkhSTUJBZjhFQlRBRApBUUgvTUE0R0ExVWREd0VCL3dRRUF3SUJCakFOQmdrcWhraUc5" +
		"dzBCQVFzRkFBT0NBUUVBTUpSaVBYeDZhNUV4CnpqOEo4SnE3WFZwNjdMMzRWd2R5UzdYTjVqQ2ZNWHli" +
		"VzZ5VHIrSWpxOEpVRHI0KzV5MkpBWmY1bFhyQ1hCK0gKdEY4MjBEd1ZFUUlEQVIrMG9GaWtSUXRYTGJn" +
		"RnMvY1hRLzY5bEE4SU5rajRIeEJHTVZidXVIQll5UWQ4Z0N5RQpGRVIxWWJLY1NvT1dxKzlabDhBYnBi" +
		"YmZvRnRnTHJDOS9CS3RiWXRmczEvYVZaa01TU2RDOVJNemxGbjVoOEw5CldFS3VUbUx3Y1BQUEpDeE9T" +
		"ZlVjb2pxRTlvOHh3c0Z4alJLQlNtRmhVTGRTOFRJMjlGTnhPL3RhalVhNFdHcE0KME1BVnRVTlQrZ2F2" +
		"Y04vZTlBQnVCZTBDaWtueVhoSXNTczhKbmlyeU91aEFFcjRZZm5Vc05iamphcEJIQnRVTgptTHFaQlpT" +
		"MlB3PT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo="
)
