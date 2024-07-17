import json
import os
from urllib.request import urlopen


def main():
    version_map = get_all_supported_k8s_patch_versions()
    update_components_json(version_map)
    update_manifest_cue(version_map)
    update_generate_windows_vhd_configuration_ps1(version_map)


def get_all_supported_k8s_patch_versions():
    """return a dict with major_minor version as key and all supported patch versions as value"""
    return { k : [f'{k}.{patch}' for patch in calculate_supported_patches(k)] for k in load_curent_k8s_major_minor_versions()}


def get_kube_proxy_latest_image_tag(version):
    with urlopen(f'https://azcu.azurewebsites.net/api/latest/oss/kubernetes/kube-proxy/v{version}', timeout=10) as resp:
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
    print("Loading current k8s major_minor versions from manifest.json file")
    dir = os.path.dirname(os.path.abspath(__file__))
    manifest_json_file = os.path.join(dir, '../../parts/linux/cloud-init/artifacts/manifest.json')
    with open(manifest_json_file) as f:
        # git rid of the ending line "#EOF"
        content = f.read()
        content = content.replace('#EOF', '')
        manifest = json.loads(content)
        versions = manifest['kubernetes']['versions']
        major_minor_versions = set()
        for version in versions:
            major_minor_versions.add('.'.join(version.split('.')[:2]))
        major_minor_versions = sorted(major_minor_versions)
        print(f"found {len(major_minor_versions)} major_minor versions: {', '.join(major_minor_versions)}")
        return major_minor_versions


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


def update_manifest_cue(version_map):
    """
    update schemas/manifest.cue, replace content between '__AUTO_ADD_START_' and 'AUTO_ADD_END__' lines with versions
    we keep 3 latest patch version so we always have the latest 2 cached in VHD even RP didn't add the latest patch version yet
    """
    print("updating manifest.cue file")
    versions = get_latest_n_patch_versions(version_map, 3)
    manifest_cue_file = os.path.join(os.path.dirname(os.path.abspath(__file__)), '../../schemas/manifest.cue')
    # open a temp file to write the updated content, then replace the original file
    tmp_file = manifest_cue_file + '.tmp'
    with open(manifest_cue_file) as f, open(tmp_file, 'w') as tmp:
        for line in f:
            if '__AUTO_ADD_START__' in line:
                tmp.write(line)
                for version in versions:
                    tmp.write(f'            "{version}",\n')
                line = f.readline()
                while '__AUTO_ADD_END__' not in line:
                    line = f.readline()
                    pass
                tmp.write(line)
            else:
                tmp.write(line)
        os.replace(tmp_file, manifest_cue_file)


def update_generate_windows_vhd_configuration_ps1(version_map):
    """
    update ../../vhdbuilder/packer/generate-windows-vhd-configuration.ps1, replace content between '__AUTO_ADD_START_' and 'AUTO_ADD_END__' lines with:
        "https://acs-mirror.azureedge.net/kubernetes/v{version}/windowszip/v{version}-1int.zip",
    where version is from versions list
    we keep 4 latest patch versions in windows vhd
    """
    print("updating generate-windows-vhd-configuration.ps1 file")
    versions = get_latest_n_patch_versions(version_map, 4)
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


def update_components_json(version_map):
    """
    update ../../parts/linux/cloud-init/artifacts/components.json
    we don't update exist images, but only add new and remove old
    keep the latest 3 patch versions for each major_minor version so we always have the latest 2 cached in VHD
    """
    components_json_file = os.path.join(os.path.dirname(os.path.abspath(__file__)), '../../parts/linux/cloud-init/artifacts/components.json')
    tmp_file = components_json_file + '.tmp'
    with open(components_json_file) as f, open(tmp_file, 'w') as tmp:
        components = json.load(f)
        kube_proxy_container_image_index = -1
        current_kube_proxy_image_tags = []
        for idx, container_image in enumerate(components['ContainerImages']):
            if container_image['downloadURL'] == 'mcr.microsoft.com/oss/kubernetes/kube-proxy:*':
                kube_proxy_container_image_index = idx 
                for img in container_image['multiArchVersions']:
                    current_kube_proxy_image_tags.append(img)
                break
        current_kube_proxy_versions = set([_img_tag_to_version(img) for img in current_kube_proxy_image_tags])

        versions_need_cache = get_latest_n_patch_versions(version_map, 3)
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
        os.replace(tmp_file, components_json_file)


def _img_tag_to_version(img_tag):
    return img_tag.split('-')[0].lstrip('v')


if __name__ == '__main__':
    main()
