# Deployment Scripts

This directory contains scripts for deploying the application to various environments.

## Prerequisites

- Ansible > 2.10 to run the playbooks and vault encryption. You can install Ansible by using the following command:

Using Homebrew:

```bash
brew install ansible
```

Using pip:

```bash
pip install ansible
```

Using apt:

```bash
sudo apt-get install ansible
```

Using Yum:

```bash
sudo yum install ansible
```

- `.vault/.dev` or `.vault.production` file for encrypting secrets.

  - Setting-up the Ansible Vault.

```bash
cd deployments
echo "secret_value" > .vault/.dev
echo "secret_value" > .vault/.production
```

## Use Cases

### Docker Compose Manifest Template

The `templates/compose.yaml.j2` file is a template for generating a Docker Compose manifest file written with Jinja2 templating engine. The file contains placeholders for environment variables that need to be replaced with actual values before running the Docker Compose command.

### Encrypting Secrets

The `templates/config.yaml.j2` file is a template for generating a configuration file written with Jinja2 templating engine. It stored config on Git repository with encrypted value using Ansible Vault.

The `encrypt_string.sh` script is a tool for encrypting the configuration value using Ansible Vault.

The `templates/.env.j2` file is a template for generating a `.env` file written with Jinja2 templating engine.

### Ansible Variables

- `app_env`: The environment of the application. It can be `development`, `staging`, or `production`. It will be placed in the generated `.env` file.
- `ansible_user`: The user to be used by Ansible. It's the same user that Ansible use to connect to the remote server through SSH.
- `ansible_python_interpreter`: The Python interpreter to be used by Ansible. It's referencing the Python interpreter path on the remote server.
- `docker_service_name`: The name of the service to be deployed. It's the same name that defined in the `docker-compose.yml` file. If you need to change it, update the SWAG reverse proxy configuration file.
- `deployment_path`: The path to the deployment directory.
- `config_file_name`: The name of the configuration file. It will be used to generate the app config yaml file.
- `image_repository`: The repository of the Docker image.
- `image_tag`: The tag of the Docker image. We left it null on the playbook file. Everytime we ran deployment, we pass extra variable to the playbook to set the image tag.
- `logging.*`: The logging configuration for the Docker Compose service.
- `domain.*`: The domain configuration for the Docker Compose service.
- `internal_config.*`: The internal configuration for generating the app config yaml file.
- `driver_config.*`: The driver configuration for generating the app config yaml file.

## How to Use

### Add New Configuration (Without Encryption)

- Define new configuration key-val in `playbook-{env}.yml` file below the `vars.internal_config` or `vars.driver_config` section.

Example:

```yaml
- name: Deploy API Development
  hosts: all
  become: true

    vars:
        internal_config:
            app:
                ....
                new_config_key: new_config_value
```

- Update the template file `templates/config.yaml.j2` with the new configuration key-val.

Example:

```yaml
internal_config:
    app:
        ....
        new_config_key: {{ internal_config.app.new_config_key | default('') }}
```

### Add New Configuration (With Encryption)

- Update the template file `templates/config.yaml.j2` with the new configuration key-val.

Example:

```yaml
internal_config:
    app:
        ....
        new_config_key: {{ internal_config.app.new_config_key | default('') }}
```

- Encrypt the new configuration value using the `encrypt_string.sh` script.

```bash
cd deployments
./encrypt_string.sh --env development --value "new_config_value"
```

Example output:

```bash
Encryption successful
encrypted_value: !vault |
          $ANSIBLE_VAULT;1.1;AES256
          33363437373839393530323133376165613562616236363239313763373535656639626231613834
          3366636435303131633139376538383534633463636537300a636561356366323634343836353832
          39313966366534386561353933326361636634653661613238393161373035356431633633656533
          3933636136666338650a313438653837343634373237336534326566303839663131386130626438
          3166
```

- Define new configuration key-val in `playbook-{env}.yml` file below the `vars.internal_config` or `vars.driver_config` section.

Example:

```yaml
- name: Deploy API Development
  hosts: all
  become: true

    vars:
        internal_config:
            app:
                ....
                new_config_key: !vault |
                    $ANSIBLE_VAULT;1.1;AES256
                    33363437373839393530323133376165613562616236363239313763373535656639626231613834
                    3366636435303131633139376538383534633463636537300a636561356366323634343836353832
                    39313966366534386561353933326361636634653661613238393161373035356431633633656533
                    3933636136666338650a313438653837343634373237336534326566303839663131386130626438
                    3166
```

## Example of Final Generated Configuration File

Example of `development` environment generated configuration file structure is like this:

```yaml
/home/konsulin/infrastructure/development/konsulin-app/
├── config.development.yaml
└── docker-compose.api.yaml
```

In the end of the Task, Ansible will run `docker-compose up -d` command to start the Docker Compose service.
