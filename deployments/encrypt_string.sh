#!/bin/bash

function usage() {
    echo "Usage: $0 --env <environment> --value <string_to_encrypt>"
    echo "Supported environments: development, production"
    exit 1
}

if ! command -v ansible-vault &>/dev/null; then
    echo "Error: ansible-vault is not installed. Please install it and try again."
    exit 1
fi

while [[ $# -gt 0 ]]; do
    key="$1"

    case $key in
    --env)
        ENV="$2"
        shift
        shift # past value
        ;;
    --value)
        VALUE="$2"
        shift # past argument
        shift # past value
        ;;
    *) # unknown option
        usage
        ;;
    esac
done

# Check if both arguments are provided
if [ -z "$ENV" ] || [ -z "$VALUE" ]; then
    usage
fi

# Set vault password file based on environment
if [ "$ENV" == "development" ]; then
    VAULT_PASSWORD_FILE=".vault/.dev"
elif [ "$ENV" == "production" ]; then
    VAULT_PASSWORD_FILE=".vault/.production"
else
    echo "Error: Unsupported environment '$ENV'. Use 'development' or 'production'."
    exit 1
fi

# Check if the vault password file exists
if [ ! -f "$VAULT_PASSWORD_FILE" ]; then
    echo "Error: Vault password file '$VAULT_PASSWORD_FILE' not found."
    exit 1
fi

# Encrypt the value using ansible-vault
echo "$VALUE" | ansible-vault encrypt_string --vault-password-file "$VAULT_PASSWORD_FILE" --stdin-name "encrypted_value"
