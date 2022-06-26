import yaml
import argparse

# String value used to replace secret data
REDACTED = 'REDACTED'

# Redact functions
def redact_bootstrap_kubeconfig_tls_token(bootstrap_kubeconfig_write_file):
    content_yaml = yaml.safe_load(bootstrap_kubeconfig_write_file['content'])
    content_yaml['users'][0]['user']['token'] = REDACTED
    bootstrap_kubeconfig_write_file['content'] = yaml.dump(content_yaml)


def redact_service_principal_secret(sp_secret_write_file: dict):
    sp_secret_write_file['content'] = REDACTED


# Maps write_file's path to the corresponding function used to redact it within cloud-config.txt
# Caller must specify write_file paths that have a mapping within this dict
PATH_TO_REDACT_FUNC = {
    '/var/lib/kubelet/bootstrap-kubeconfig': redact_bootstrap_kubeconfig_tls_token,
    '/etc/kubernetes/sp.txt': redact_service_principal_secret
}


def perform_redact(cloud_config_path, target_paths, output_path):
    for target_path in target_paths:
        if target_path not in PATH_TO_REDACT_FUNC:
            raise ValueError(f'Target path: {target_path} is not recognized as a redactable write_file path')
 
    with open(cloud_config_path, 'r') as f:
        cloud_config_data = f.read()
    cloud_config = yaml.safe_load(cloud_config_data)

    for write_file in cloud_config['write_files']:
        if write_file['path'] in target_paths:
            target_path = write_file['path']
            target_paths.remove(target_path)

            print(f'Redacting secrets from write_file: {target_path}')
            PATH_TO_REDACT_FUNC[target_path](write_file)
        
        if len(target_paths) == 0:
            break
        

    print(f'Dumping redacted cloud-config to: {output_path}')
    with open(output_path, 'w+') as output_file:
        output_file.write(yaml.dump(cloud_config))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(
        description=f'Command line utility used to redact secrets from write_file definitions for \
            {list(PATH_TO_REDACT_FUNC.keys())} within cloud-config.txt. These secrets must be redacted as \
            cloud-conifg.txt will be collected by the WALinuxAgent.')
    parser.add_argument(
        "--cloud-config-path",
        required=True, 
        type=str, 
        help='Path to cloud-config')
    parser.add_argument(
        "--target-paths",
        nargs='+',
        required=True,
        help=f'Paths of the targeted write_file definitions to redact secrets from. Must be one of: {list(PATH_TO_REDACT_FUNC.keys())}')
    parser.add_argument(
        "--output-path",
        required=True,  
        type=str, 
        help='Path to the newly redacted cloud-config')
    
    args = parser.parse_args()
    perform_redact(args.cloud_config_path, set(args.target_paths), args.output_path)

