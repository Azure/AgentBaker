package main

import (
	"encoding/json"
	"fmt"
	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
)

func main() {
	config := nbcontractv1.Configuration{
		CustomLinuxOsConfig: &nbcontractv1.CustomLinuxOSConfig{
			SysctlConfig: &nbcontractv1.SysctlConfig{},
		},
	}
	netcoreval := int32(4)
	config.CustomLinuxOsConfig.SysctlConfig.NetCoreOptmemMax = &netcoreval
	jsonData, err := json.Marshal(config)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println(string(jsonData))

	var deserialized nbcontractv1.Configuration
	jsonString := "{\"custom_linux_os_config\":{\"sysctl_config\":{\"NetCoreOptmemMax\":4}}}"
	err = json.Unmarshal([]byte(jsonString), &deserialized)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Print the struct
	fmt.Printf("%+v\n", deserialized)
}
