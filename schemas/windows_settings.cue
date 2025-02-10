
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

#WindowsSettings: {
  WindowsRegistryKeys: #WindowsRegistryKeys
}

#WindowsSettings