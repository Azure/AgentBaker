
#WindowsRegistryKey: {
	Comment:       	    string
	WindowsSkuMatch:  	string
	Path:           	 	string
	Name:   						string
	Value:        			string
	Type: 							string
}

#WindowsRegistryKeys: [...#WindowsRegistryKey]

#WindowsSettings: {
	WindowsRegistryKeys: #WindowsRegistryKeys
}

#WindowsSettings