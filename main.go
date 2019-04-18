package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/ovotech/cloud-key-rotator/cmd"
	"github.com/ovotech/cloud-key-rotator/pkg/config"
	"github.com/ovotech/cloud-key-rotator/pkg/rotate"
)

type MyEvent struct {
	Name string `json:"name"`
}

func HandleRequest(ctx context.Context, name MyEvent) (string, error) {
	//TODO: get config from
	// cmd.Execute()
	var c config.Config
	rotate.Rotate("", "", "", c)
	return "", nil
}

func main() {
	if isLambda() {
		lambda.Start(HandleRequest)
	} else {
		cmd.Execute()
	}
}

func isLambda() (isLambda bool) {
	return
}
