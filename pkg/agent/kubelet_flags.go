package agent

import (
	"encoding/base64"
	"encoding/json"
)

func getKubeletFlagsToOmit(features map[string]string) string {
	encoded := features["KUBELET_FLAGS_TO_OMIT"]
	if len(encoded) > 1024 {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(encoded)
		if err != nil {
			return ""
		}
	}
	var flags []string
	if json.Unmarshal(decoded, &flags) != nil || len(flags) == 0 || len(flags) > 16 {
		return ""
	}
	return base64.RawStdEncoding.EncodeToString(decoded)
}
