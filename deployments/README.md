# Deployment Scripts

This directory contains scripts for deploying the application to various environments.

## Prerequisites

- Ansible > 2.10 to run the playbooks and vault encryption.
- `.vault/.dev` or `.vault.production` file for encrypting secrets.

## Use Cases

### Docker Compose Manifest Template

The `templates/compose.yaml.j2` file is a template for generating a Docker Compose manifest file written with Jinja2 templating engine. The file contains placeholders for environment variables that need to be replaced with actual values before running the Docker Compose command.

### Encrypting Secrets

The `templates/config.yaml.j2` file is a template for generating a configuration file written with Jinja2 templating engine. It stored config on Git repository with encrypted value using Ansible Vault.

## How to Use

### Add New Configuration (Without Encryption)

- Define new configuration key-val in `deploy-{env}.yml` file below the `vars.internal_config` or `vars.driver_config` section.

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
bash encrypt_string.sh --env development --value "new_config_value"
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

- Define new configuration key-val in `deploy-{env}.yml` file below the `vars.internal_config` or `vars.driver_config` section.

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
