# Datadog GCP Integration Example

## Pre-requisites

In order to rotate a GCP key that's being used for Datadog's GCP integration,
you'll need:

1. A Datadog API key and app key.
2. An existing integration

## Configuration

In order to rotate the Datadog service account key you need to provide the
project that the service account has permissions for, and the full email
address of the service account. You also need to supply the API key and app
key for Datadog authentication. An example is given below:

```json
    "AccountKeyLocations": [
        {
            "ServiceAccountName": "my_datadog_service_account",
            "DatadogGCPIntegration": [
                {
                  "Project": "my_gcp_project",
                  "ClientEmail": "my_datadog_service_account@my_gcp_project.iam.gserviceaccount.com"
                }
            ]
        }
    ],
    "Credentials": {
        "Datadog": {
            "APIKey": "my_datadog_api_key",
            "AppKey": "my_datadog_app_key"
        }
    }
```
