package scenario

import (
	"fmt"
	"strings"

	"github.com/Azure/agentbaker/e2e/config"
)

const rcv1pCertTrustScript = `set -euo pipefail
download_dir=$1
trust_dir=$2
bundle=$3
extension=$4
shopt -s nullglob
certs=("$download_dir"/*.crt)
if [ "${#certs[@]}" -eq 0 ]; then
    echo "No downloaded RCV1P certificates found in $download_dir" >&2
    exit 1
fi
for cert in "${certs[@]}"; do
    name=${cert##*/}
    installed="$trust_dir/${name%.crt}.$extension"
    expected=$(openssl x509 -in "$cert" -noout -sha256 -fingerprint)
    actual=$(openssl x509 -in "$installed" -noout -sha256 -fingerprint)
    if [ "$expected" != "$actual" ]; then
        echo "RCV1P certificate fingerprint mismatch: $cert -> $installed" >&2
        exit 1
    fi
    openssl verify -CAfile "$bundle" -no-CApath "$cert"
done`

func rcv1pCertTrustCommand(s *Scenario) string {
	bundle := "/etc/ssl/certs/ca-certificates.crt"
	extension := "crt"
	switch s.VHD.OS {
	case config.OSMariner, config.OSAzureLinux, config.OSACL:
		bundle = "/etc/pki/tls/certs/ca-bundle.crt"
	case config.OSFlatcar:
		extension = "pem"
	}
	// The script is passed as one literal argument; certificate filenames are never evaluated as shell code.
	script := "'" + strings.ReplaceAll(rcv1pCertTrustScript, "'", "'\"'\"'") + "'"
	return fmt.Sprintf("sudo bash -c %s -- /root/AzureCACertificates %s %s %s",
		script, rcv1pTrustStoreDir(s), bundle, extension)
}
