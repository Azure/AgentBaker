import json
import os
import urllib
import urllib.request

COMPONENTS_JSON_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), '../../parts/linux/cloud-init/artifacts/components.json')


def main():
    version_map = get_all_supported_k8s_patch_versions()
    current_kube_proxy_image_tags = load_curent_kube_proxy_image_tags()
    update_components_json_file_kubernetes_binaries(version_map)
    update_components_json_file_kube_proxy_image_tags(version_map, current_kube_proxy_image_tags)
    update_generate_windows_vhd_configuration_ps1(version_map)


def get_all_supported_k8s_patch_versions():
    """return a dict with major_minor version as key and all supported patch versions as value"""
    return { k : [f'{k}.{patch}' for patch in calculate_supported_patches(k)] for k in load_curent_k8s_major_minor_versions()}


def get_kube_proxy_latest_image_tag(version):
    with urllib.request.urlopen(f'https://azcu.azurewebsites.net/api/latest/oss/kubernetes/kube-proxy/v{version}', timeout=10) as resp:
        data = resp.read().decode('utf-8')
        return data


# Get the latest patch version for the given major.minor version, e.g. 1.30
def get_kube_proxy_latest_patch(major_minor_version):
    latest_img_tag = get_kube_proxy_latest_image_tag(major_minor_version)
    v = latest_img_tag.split('-')[0]
    v_parts = v.split('.')
    if len(v_parts) != 3:
        raise ValueError(f'Invalid version format: {v}')
    print(f'latest patch version for {major_minor_version} is {v}')
    return int(v_parts[2])


# Load the current k8s major_minor versions from manifest.json file
def load_curent_k8s_major_minor_versions():
    print(f'Loading current k8s major_minor versions from {COMPONENTS_JSON_FILE}')
    with open(COMPONENTS_JSON_FILE) as f:
        components = json.load(f)
        major_minor_versions = set()
        for package in components['Packages']:
            if package['name'] == 'kubernetes-binaries':
                for version in package['downloadURIs']['default']['current']['versions']:
                    major_minor_versions.add('.'.join(version.split('.')[:2]))
                break
        major_minor_versions = sorted(major_minor_versions)
        print(f"found {len(major_minor_versions)} major_minor versions: {', '.join(major_minor_versions)}")
        return major_minor_versions


def load_curent_kube_proxy_image_tags():
    with open(COMPONENTS_JSON_FILE) as f:
        components = json.load(f)
        current_kube_proxy_image_tags = []
        for container_image in components['ContainerImages']:
            if container_image['downloadURL'] == 'mcr.microsoft.com/oss/kubernetes/kube-proxy:*':
                for img in container_image['multiArchVersions']:
                    current_kube_proxy_image_tags.append(img)
                break
        return current_kube_proxy_image_tags


# we keep 3 latest patch version so we always have the latest 2 cached in VHD
# even RP didn't add the latest patch version yet
def update_components_json_file_kubernetes_binaries(version_map):
    print(f"updating kubernetes-binaries in {COMPONENTS_JSON_FILE}")
    versions = get_latest_n_patch_versions(version_map, 3)
    for v in versions:
        _make_sure_k8s_tar_exist(v)
    tmp_file = COMPONENTS_JSON_FILE + '.tmp'
    with open(COMPONENTS_JSON_FILE) as f, open(tmp_file, 'w') as tmp:
        components = json.load(f)
        kubernetes_binaries_index = -1
        for idx, package in enumerate(components['Packages']):
            if package['name'] == 'kubernetes-binaries':
                kubernetes_binaries_index = idx
        components['Packages'][kubernetes_binaries_index]['downloadURIs']['default']['current']['versions'] = versions
        json.dump(components, tmp, indent=2)
        tmp.write('\n')
        os.replace(tmp_file, COMPONENTS_JSON_FILE)


def update_components_json_file_kube_proxy_image_tags(version_map, current_kube_proxy_image_tags):
    print(f"updating kubernetes-proxy in {COMPONENTS_JSON_FILE}")
    versions_need_cache = get_latest_n_patch_versions(version_map, 3)
    current_kube_proxy_versions = set([_img_tag_to_version(img) for img in current_kube_proxy_image_tags])
    tmp_file = COMPONENTS_JSON_FILE + '.tmp'
    with open(COMPONENTS_JSON_FILE) as f, open(tmp_file, 'w') as tmp:
        components = json.load(f)
        kube_proxy_container_image_index = -1
        for idx, container_image in enumerate(components['ContainerImages']):
            if container_image['downloadURL'] == 'mcr.microsoft.com/oss/kubernetes/kube-proxy:*':
                kube_proxy_container_image_index = idx 
                break

        goal_image_tags = []
        # delete patches that are not latest 3 patches
        for img in current_kube_proxy_image_tags:
            if _img_tag_to_version(img) in versions_need_cache:
                goal_image_tags.append(img)
        # add new if not exist
        for v in versions_need_cache:
            if v not in current_kube_proxy_versions:
                latest_img = get_kube_proxy_latest_image_tag(v)
                goal_image_tags.append(latest_img)
        goal_image_tags.sort(key=lambda x: tuple(map(int, _img_tag_to_version(x).split('.'))))

        components['ContainerImages'][kube_proxy_container_image_index]['multiArchVersions'] = goal_image_tags
        json.dump(components, tmp, indent=2)
        tmp.write('\n')
        os.replace(tmp_file, COMPONENTS_JSON_FILE)


def calculate_supported_patches(major_minor_version):
    """
    we start to support all patch versions since 1.27.13, 1.28.9, 1.29.4 and 1.30+ clusters,
    before that, we support:
    1.27: [1, 3, 7, 9, 13]
    1.28: [0, 3, 5, 9]
    1.29: [0, 2, 4]
    """
    part_supported_patches = {
        '1.27': [1, 3, 7, 9, 13],
        '1.28': [0, 3, 5, 9],
        '1.29': [0, 2, 4],
    }
    # extend to latest patch version
    latest_patch = get_kube_proxy_latest_patch(major_minor_version)
    if major_minor_version in part_supported_patches:
        supported_patches = part_supported_patches[major_minor_version]
        for patch in range(supported_patches[-1] + 1, latest_patch + 1):
            supported_patches.append(patch)
    else:
        supported_patches = list(range(0, latest_patch + 1))
    return supported_patches


def get_latest_n_patch_versions(version_map, n):
    """return the latest n patch versions from version_map"""
    versions = []
    for patches in version_map.values():
        lastest_n = patches[-n:]
        versions.extend(lastest_n)
    return sorted(versions, key=lambda x: tuple(map(int, x.split('.'))))


def update_generate_windows_vhd_configuration_ps1(version_map):
    """
    update ../../vhdbuilder/packer/generate-windows-vhd-configuration.ps1, replace content between '__AUTO_ADD_START_' and 'AUTO_ADD_END__' lines with:
        "https://acs-mirror.azureedge.net/kubernetes/v{version}/windowszip/v{version}-1int.zip",
    where version is from versions list
    we keep 4 latest patch versions in windows vhd
    """
    print("updating generate-windows-vhd-configuration.ps1 file")
    versions = get_latest_n_patch_versions(version_map, 4)
    for v in versions:
        _make_sure_windowszip_exist(v)
    ps1_file = os.path.join(os.path.dirname(os.path.abspath(__file__)), '../../vhdbuilder/packer/generate-windows-vhd-configuration.ps1')
    # open a temp file to write the updated content, then replace the original file
    tmp_file = ps1_file + '.tmp'
    with open(ps1_file) as f, open(tmp_file, 'w') as tmp:
        for line in f:
            if '__AUTO_ADD_START__' in line:
                tmp.write(line)
                tmp.write('        ')
                tmp.write(',\n        '.join([f'"https://acs-mirror.azureedge.net/kubernetes/v{version}/windowszip/v{version}-1int.zip"' for version in versions]))
                line = f.readline()
                while '__AUTO_ADD_END__' not in line:
                    line = f.readline()
                    pass
                tmp.write('\n')
                tmp.write(line)
            else:
                tmp.write(line)
        os.replace(tmp_file, ps1_file)


def _img_tag_to_version(img_tag):
    return img_tag.split('-')[0].lstrip('v')

def _make_sure_k8s_tar_exist(version):
    print(f'checking k8s tar file for version {version}')
    url = f'https://acs-mirror.azureedge.net/kubernetes/v{version}/binaries/kubernetes-node-linux-amd64.tar.gz'
    if _http_head_request_status_code(url) != 200:
        raise ValueError(f'k8s tar file not found for version {version}')

def _make_sure_windowszip_exist(version):
    print(f'checking windows zip file for version {version}')
    url = f'https://acs-mirror.azureedge.net/kubernetes/v{version}/windowszip/v{version}-1int.zip'
    if _http_head_request_status_code(url) != 200:
        raise ValueError(f'windows zip file not found for version {version}')

def _http_head_request_status_code(url):
    req = urllib.request.Request(url, method='HEAD')
    resp = urllib.request.urlopen(req, timeout=10)
    return resp.status


if __name__ == '__main__':
    main()
