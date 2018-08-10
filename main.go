package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	awsiam "github.com/aws/aws-sdk-go/service/iam"
	"github.com/kelseyhightower/envconfig"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iam/v1"
)

const (
	envConfigPrefix         = "kd"
	gcpTimeFormat           = "2006-01-02T15:04:05Z"
	awsTimeFormat           = "2006-01-02 15:04:05 +0000 UTC"
	project                 = "cepheus-202811"
	ageThresholdMins        = 43200 //30 days
	datadogURL              = "https://api.datadoghq.com/api/v1/series?api_key="
	gcpServiceAccountPrefix = "serviceAccounts/"
	gcpServiceAccountSuffix = "@"
	gcpKeyPrefix            = "keys/"
	gcpKeySuffix            = ""
)

//Specification struct for keylseyhightower's envconfigs
type Specification struct {
	AgeMetricGranularity string `default:"day"`
	GcpProject           string `required:"true"`
	DatadogAPIKey        string `required:"true"`
}

type key struct {
	keyAge float64
	name   string
	id     string
}

func main() {
	var spec Specification
	err := envconfig.Process(envConfigPrefix, &spec)
	check(err)
	keys := make([]key, 0)
	keys = appendSlice(keys, gcpKeys(spec))
	keys = appendSlice(keys, awsKeys(spec))
	postMetric(keys, spec)
}

func awsKeys(spec Specification) (keys []key) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("eu-west-2")},
	)
	check(err)
	// Create a IAM service client.
	svc := awsiam.New(sess)
	result, err := svc.ListAccessKeys(&awsiam.ListAccessKeysInput{
		MaxItems: aws.Int64(5),
		UserName: aws.String("key-dater"),
	})
	check(err)
	for _, awsKey := range result.AccessKeyMetadata {
		keys = append(keys,
			key{adjustAgeScale(minsSinceCreation(*awsKey.CreateDate), spec),
				*awsKey.UserName, *awsKey.AccessKeyId})
	}
	return
}

func appendSlice(keys, keysToAdd []key) []key {
	for _, keyToAdd := range keysToAdd {
		keys = append(keys, keyToAdd)
	}
	return keys
}

func gcpKeys(spec Specification) (keys []key) {
	ctx := context.Background()
	client, err := google.DefaultClient(ctx, iam.CloudPlatformScope)
	check(err)
	service, err := iam.New(client)
	check(err)
	for _, acc := range serviceAccounts(spec.GcpProject, *service) {
		for _, gcpKey := range gcpServiceAccountKeys(project, acc.Email, *service) {
			//only iterate over keys that have a mins-to-expiry, or are of an age,
			// above a specific threshold, to differentiate between GCP-managed
			// and User-managed keys:
			// https://cloud.google.com/iam/docs/understanding-service-accounts
			keyAge := minsSinceCreation(parseTime(gcpTimeFormat, gcpKey.ValidAfterTime))
			keyMinsToExpiry := minsSinceCreation(parseTime(gcpTimeFormat,
				gcpKey.ValidBeforeTime))
			if math.Abs(keyMinsToExpiry) > ageThresholdMins ||
				keyAge > ageThresholdMins {
				keys = append(keys, key{adjustAgeScale(keyAge, spec),
					subString(gcpKey.Name, gcpServiceAccountPrefix,
						gcpServiceAccountSuffix),
					subString(gcpKey.Name, gcpKeyPrefix, gcpKeySuffix)})
			}
		}
	}
	return
}

func adjustAgeScale(keyAge float64, spec Specification) (adjustedAge float64) {
	switch spec.AgeMetricGranularity {
	case "day":
		adjustedAge = keyAge / 60 / 24
	case "hour":
		adjustedAge = keyAge / 60
	case "min":
		//do nothing, already in mins
	default:
		panic("Unsupported age metric granularity: " +
			spec.AgeMetricGranularity)
	}
	return
}

func subString(str string, start string, end string) (result string) {
	startIndex := strings.Index(str, start)
	if startIndex != -1 {
		startIndex += len(start)
		endIndex := len(str)
		if len(end) > 0 {
			endIndex = strings.Index(str, end)
		}
		result = str[startIndex:endIndex]
	} else {
		//TODO: log this, panic
	}
	return
}

func postMetric(keys []key, spec Specification) {
	url := strings.Join([]string{datadogURL, spec.DatadogAPIKey}, "")
	for _, key := range keys {
		var jsonString = []byte(`{ "series" :[{"metric":"key-dater.age","points":[[` +
			strconv.FormatInt(time.Now().Unix(), 10) +
			`, ` + strconv.FormatFloat(key.keyAge, 'f', 2, 64) +
			`]],"type":"count","tags":["team:cepheus","environment:prod","key:` +
			key.name + `","id:` + key.id + `"]}]}`)
		fmt.Println(string(jsonString))
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonString))
		check(err)
		req.Header.Set("Content-type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)
		check(err)
		defer resp.Body.Close()
		if resp.StatusCode != 202 {
			body, err := ioutil.ReadAll(resp.Body)
			check(err)
			bodyString := string(body)
			fmt.Println(string(bodyString))
			panic("non-202 status code (" + strconv.Itoa(resp.StatusCode) +
				") returned by Datadog")
		}
	}
}

func parseTime(timeFormat, timeString string) (then time.Time) {
	then, err := time.Parse(timeFormat, timeString)
	check(err)
	return
}

func minsSinceCreation(then time.Time) (minsSinceCreation float64) {
	// then, err := time.Parse(timeFormat, timestamp)
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	duration := time.Since(then)
	minsSinceCreation = duration.Minutes()
	return
}

//check panics if error is not nil
func check(e error) {
	if e != nil {
		panic(e.Error())
	}
}

func serviceAccounts(project string, service iam.Service) (accs []*iam.ServiceAccount) {
	res, err := service.Projects.ServiceAccounts.List(fmt.Sprintf("projects/%s", project)).Do()
	check(err)
	accs = res.Accounts
	return
}

func gcpServiceAccountKeys(project, email string, service iam.Service) (keys []*iam.ServiceAccountKey) {
	res, err := service.Projects.ServiceAccounts.Keys.List(fmt.Sprintf("projects/%s/serviceAccounts/%s", project, email)).Do()
	check(err)
	keys = res.Keys
	return
}
