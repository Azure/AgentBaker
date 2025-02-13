
#WindowsRegistryKey: {
  Comment:            string
  WindowsSkuMatch:    string
  Path:               string
  Name:               string
  Value:              string
  Operation?:         string // default is "replace". Options are "bor" (bitwise or) and "replace".
  Type:               string
}

#WindowsRegistryKeys: [...#WindowsRegistryKey]

#WindowsPatch: {
	id: string
	url: string
}

#WindowsPatches: [...#WindowsPatch]

#WindowsBaseVersion: {
  os_disk_size?:      string
  base_image_sku:     string,
  base_image_version: string
  windows_image_name: string
  patches_to_apply:   #WindowsPatches
}

#WindowsBaseVersions: {
  "2019": #WindowsBaseVersion
  "2019-containerd": #WindowsBaseVersion
  "2022-containerd": #WindowsBaseVersion
  "2022-containerd-gen2": #WindowsBaseVersion
  "23H2": #WindowsBaseVersion
  "23H2-gen2": #WindowsBaseVersion
}

#WindowsSettings: {
  WindowsRegistryKeys: #WindowsRegistryKeys
  WindowsBaseVersions: #WindowsBaseVersions
}
  
#WindowsSettings