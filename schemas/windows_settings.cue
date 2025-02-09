
#WindowsRegistryKey: {
	Comment:           	string
	WindowsSkuMatch:  	string
	Path:           	 	string
	Name:   						string
	Value?:        			string  // defaults to "1"
	Type?: 							string  // defaults to DWORD
}

#WindowsRegistryKeys: [...#WindowsRegistryKey]

#WindowsSettings: {
	WindowsRegistryKeys: #WindowsRegistryKeys
}

#WindowsSettings