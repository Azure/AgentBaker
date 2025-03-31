import yaml
import argparse

REDACTED = 'REDACTED'

def redact_bootstrap_kubeconfig_tls_token(bootstrap_kubeconfig_write_file):
    content_yaml = yaml.safe_load(bootstrap_kubeconfig_write_file['content'])
    content_yaml['users'][0]['user']['token'] = REDACTED
    bootstrap_kubeconfig_write_file['content'] = yaml.dump(content_yaml)


def redact_service_principal_secret(sp_secret_write_file):
    sp_secret_write_file['content'] = REDACTED


PATH_TO_REDACT_FUNC = {
    '/var/lib/kubelet/bootstrap-kubeconfig': redact_bootstrap_kubeconfig_tls_token,
    '/etc/kubernetes/sp.txt': redact_service_principal_secret
}


def redact_cloud_config(cloud_config_path, output_path):
    target_paths = set(PATH_TO_REDACT_FUNC.keys())

    with open(cloud_config_path, 'r') as f:
        cloud_config_data = f.read()
    cloud_config = yaml.safe_load(cloud_config_data)

    for write_file in cloud_config['write_files']:
        if write_file['path'] in target_paths:
            target_path = write_file['path']
            target_paths.remove(target_path)

            print('Redacting secrets from write_file: ' + target_path)
            PATH_TO_REDACT_FUNC[target_path](write_file)

        if len(target_paths) == 0:
            break


    print('Dumping redacted cloud-config to: ' + output_path)
    with open(output_path, 'w+') as output_file:
        output_file.write(yaml.dump(cloud_config))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(
        description='Command line utility used to redact secrets from write_file definitions for ' +
            str([", ".join(PATH_TO_REDACT_FUNC)]) + ' within a specified cloud-config.txt. \
            These secrets must be redacted before cloud-config.txt can be collected for logging.')
    parser.add_argument(
        "--cloud-config-path",
        required=True,
        type=str,
        help='Path to cloud-config.txt to redact')
    parser.add_argument(
        "--output-path",
        required=True,
        type=str,
        help='Path to the newly generated cloud-config.txt with redacted secrets')

    args = parser.parse_args()
    redact_cloud_config(args.cloud_config_path, args.output_path)