
#WindowsRegistryKey: {
  Comment:            string
  WindowsSkuMatch:    string
  Path:               string
  Name:               string
  Value:              string
  Operation?:         string
  Type:               string
}

#WindowsRegistryKeys: [...#WindowsRegistryKey]

#WindowsSettings: {
  WindowsRegistryKeys: #WindowsRegistryKeys
}

#WindowsSettings