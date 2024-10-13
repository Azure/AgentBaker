@minLength(5)
@maxLength(50)

@description('Provide a globally unique name of your Azure Container Registry')
param acrName string 

resource acrResource 'Microsoft.ContainerRegistry/registries@2023-01-01-preview' = {
  name: acrName
  location: resourceGroup().location
  sku: {
    name: 'Premium'
  }
  properties: {
    adminUserEnabled: false
    anonymousPullEnabled: true
  }
}

resource cacheRule 'Microsoft.ContainerRegistry/registries/cacheRules@2023-01-01-preview' = {
  name: 'aks-managed-rule'
  parent: acrResource
  properties: {
    sourceRepository: 'mcr.microsoft.com/*'
    targetRepository: 'aks/*'
  }
}

