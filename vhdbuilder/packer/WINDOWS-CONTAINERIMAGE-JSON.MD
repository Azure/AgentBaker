# Using the Windows Container Image JSON Blob

This guide is for developers who want to manually download and inspect the JSON blob used in `produce-packer-settings.sh` for Windows VHD builds. 
The blob contains image URLs for various Windows SKUs and is used to configure builds dynamically.

## How to download the JSON blob
⚠️ Access to the WCCT AgentBaker Storage Account is restricted. If you need access, please contact [yuazhang@microsoft.com](mailto:yuazhang@microsoft)
1. Navigate to the WCCT AgentBaker Storage Account via Azure Portal
2. Browse to:  
   `Blob containers > simship > 2025 > 04B` *(or later service releases)*  
3. Download the `payload.json` file.

## 📄 Here's an example JSON Format 
```json
{
    "datetime": "20250407",
    "service-release": "04B",
    "images": [
        {
            "name": "WINDOWS_2019_BASE_IMAGE_URL",
            "value": "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/04B/ws2019/2019-datacenter-core-smalldisk-sim.vhd"
        },
        {
            "name": "WINDOWS_2019_CORE_IMAGE_URL",
            "value": "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/04B/ws2019/CONTAINERS/servercore.tar"
        },
        {
            "name": "WINDOWS_2019_NANO_IMAGE_URL",
            "value": "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/04B/ws2019/CONTAINERS/nanoserver.tar"
        },
        {
            "name": "WINDOWS_2022_BASE_IMAGE_URL",
            "value": "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/04B/ws2022/2022-datacenter-core-smalldisk-sim.vhd"
        },
        {
            "name": "WINDOWS_2022_CORE_IMAGE_URL",
            "value": "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/04B/ws2022/CONTAINERS/servercore.tar"
        },
        {
            "name": "WINDOWS_2022_NANO_IMAGE_URL",
            "value": "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/04B/ws2022/CONTAINERS/nanoserver.tar"
        },
        {
            "name": "WINDOWS_2022_GEN2_BASE_IMAGE_URL",
            "value": "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/04B/ws2022/GEN2/2022-datacenter-core-smalldisk-g2-sim.vhd"
        },
        {
            "name": "WINDOWS_2025_BASE_IMAGE_URL",
            "value": "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/04B/ws2025/2025-datacenter-core-smalldisk-sim.vhd"
        },
        {
            "name": "WINDOWS_2025_CORE_IMAGE_URL",
            "value": "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/04B/ws2025/CONTAINERS/servercore.tar"
        },
        {
            "name": "WINDOWS_2025_NANO_IMAGE_URL",
            "value": "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/04B/ws2025/CONTAINERS/nanoserver.tar"
        },
        {
            "name": "WINDOWS_2025_GEN2_BASE_IMAGE_URL",
            "value": "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/04B/ws2025/GEN2/2025-datacenter-core-smalldisk-g2-sim.vhd"
        },
        {
            "name": "WINDOWS_23H2_BASE_IMAGE_URL",
            "value": "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/04B/ws23H2/23h2-datacenter-core-sim.vhd"
        },
        {
            "name": "WINDOWS_23H2_GEN2_BASE_IMAGE_URL",
            "value": "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/04B/ws23H2/GEN2/23h2-datacenter-core-g2-sim.vhd"
        }
    ]
}
```

## 🛠️ How It's Used
The JSON blob is parsed in `produce-packer-settings.sh` to extract image URLs based on the `WINDOWS_SKU`. These URLs are then used to configure the VHD build process dynamically, avoiding hardcoded values.
