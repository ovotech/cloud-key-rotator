#!/usr/bin/env bash

set -e
set -o pipefail

VERSION=$1
readonly VERSION


REGISTRY_URL=$(curl --fail -sL "https://$TF_REGISTRY_HOST/.well-known/terraform.json" | jq -r '."modules.v1"')

if [[ "$REGISTRY_URL" == "" ]]; then
    echo "Failed to find registry API"
    exit 2
fi

EXISTING_VERSION=$(curl "${REGISTRY_URL}${MODULE_NAME}/versions" | jq -c '.modules[0].versions[] | select(.version == '\""$VERSION"\"')')

if [[ -n "$EXISTING_VERSION" ]]; then
    echo "Version $VERSION already exists, skipping"
    circleci-agent step halt
fi
