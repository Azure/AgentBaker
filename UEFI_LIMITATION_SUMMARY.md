# UEFI Secure Boot Certificate Limitations in AKS

## Summary
Custom UEFI secure boot certificates are **NOT SUPPORTED** in AKS through public APIs.

## Issues with Our Previous Approach

### 1. ARM Template Issues
- ❌ `nodeImageVersion` is not a valid property in `agentPoolProfiles`
- ❌ `additionalSignatures` is not supported in public ARM schema
- ❌ UEFI certificate format expects resource ID, not raw certificate bytes

### 2. CLI Limitations
- ❌ `az aks create` has no parameter for custom UEFI certificates
- ❌ Custom node images must be passed via internal APIs or extensions

### 3. SIG Image Creation
- ❌ `az sig image-version create` doesn't support `--security-type` or `--uefi-settings`
- ❌ UEFI settings apply at VM creation time, not image creation time

## What Actually Works in Public AKS

### Standard Trusted Launch (Supported)
```bash
az aks create \
  --resource-group myRG \
  --name myCluster \
  --enable-trusted-launch
```

### Custom Node Images (Limited Support)
- Only through AKS preview CLI with `--node-image-version`
- No custom UEFI certificate support

## AgentBaker Context

AgentBaker is Microsoft's **internal** VHD building tool. The UEFI certificate functionality you see here is for:

1. **Microsoft's internal image building** - Creating base AKS node images
2. **Internal API surfaces** - Not exposed to public customers
3. **Testing frameworks** - Internal validation only

## Recommendation

For custom secure boot requirements:
1. Use Microsoft's pre-signed images with standard Trusted Launch
2. Contact Microsoft support for enterprise-specific requirements
3. Consider alternative security approaches (encryption at rest, etc.)

## Technical Reality

The `publish-imagecustomizer-image.sh` script we were working with is designed for Microsoft's internal VHD publishing pipeline, not for external customer use.
