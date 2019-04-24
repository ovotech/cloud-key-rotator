package main

import (
	"context"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/ovotech/cloud-key-rotator/cmd"
	"github.com/ovotech/cloud-key-rotator/pkg/config"
	"github.com/ovotech/cloud-key-rotator/pkg/rotate"
)

// MyEvent type
type MyEvent struct {
	Name string `json:"name"`
}

//HandleRequest allows cloud-key-rotator to be used in the Lambda program model
func HandleRequest(ctx context.Context, name MyEvent) (string, error) {
	var c config.Config
	var err error
	status := "fail"
	if c, err = config.GetConfigFromAWSSecretManager(
		getEnv("CKR_SECRET_CONFIG_NAME", "ckr-config"),
		getEnv("CKR_CONFIG_TYPE", "json")); err != nil {
		return status, err
	}
	if err = rotate.Rotate("", "", "", c); err == nil {
		status = "success"
	}
	return status, err
}

func main() {
	if isLambda() {
		lambda.Start(HandleRequest)
	} else {
		cmd.Execute()
	}
}

//isLambda returns true if the AWS_LAMBDA_FUNCTION_NAME env var is set
func isLambda() (isLambda bool) {
	return len(os.Getenv("AWS_LAMBDA_FUNCTION_NAME")) > 0
}

//getEnv returns the value of the env var matching the key, if it exists, and
// the value of fallback otherwise
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
