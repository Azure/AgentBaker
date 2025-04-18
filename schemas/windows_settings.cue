
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
  comment?:           string
  os_disk_size:      string
  base_image_sku:     string,
  base_image_version: string
  windows_image_name: string
  patches_to_apply:   #WindowsPatches
}

#WindowsComments: [...string]


#WindowsBaseVersions: {
  "2019-containerd": #WindowsBaseVersion
  "2022-containerd": #WindowsBaseVersion
  "2022-containerd-gen2": #WindowsBaseVersion
  "23H2": #WindowsBaseVersion
  "23H2-gen2": #WindowsBaseVersion
  "2025": #WindowsBaseVersion
  "2025-gen2": #WindowsBaseVersion
}

#WindowsDefenderInfo: {
  DefenderUpdateUrl:     string,
  DefenderUpdateInfoUrl: string
}

#WindowsSettings: {
  WindowsComments?:    #WindowsComments
  WindowsDefenderInfo: #WindowsDefenderInfo
  WindowsRegistryKeys: #WindowsRegistryKeys
  WindowsBaseVersions: #WindowsBaseVersions
}
  
#WindowsSettings