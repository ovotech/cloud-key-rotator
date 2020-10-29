#!/bin/bash

set +e

KEY_ROTATE_VERSION=$1
OUTPUT_FILENAME=$2

curl -L --output "$OUTPUT_FILENAME" "https://github.com/ovotech/cloud-key-rotator/releases/download/v${KEY_ROTATE_VERSION}/cloud-key-rotator_${KEY_ROTATE_VERSION}_cloudfunction.zip" &> curl_output

if [ $? -ne 0 ]; then
    cat curl_output
    exit 1
fi

echo "{\"output_filename\": \"${OUTPUT_FILENAME}\"}"
