package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/ovotech/cloud-key-client"
)

const (
	datadogURL          = "https://api.datadoghq.com/api/v1/series?api_key="
	envConfigPrefix     = "kd"
	gcpAgeThresholdMins = 43200 //30 days
)

//Specification struct for keylseyhightower's envconfigs
type Specification struct {
	AgeMetricGranularity string `default:"day"`
	AwsRegions           string
	GcpProject           string
	IncludeAwsUserKeys   bool
	DatadogAPIKey        string
	RotationMode         bool
	Providers            []string `required:"true"`
}

func main() {
	var spec Specification
	err := envconfig.Process(envConfigPrefix, &spec)
	check(err)
	providers := make([]keys.Provider, 0)
	for _, provider := range spec.Providers {
		if strings.HasPrefix(provider, "gcp") && strings.Contains(provider, ":") {
			gcpProject := strings.Split(provider, ":")[1]
			providers = append(providers, keys.Provider{GcpProject: gcpProject,
				Provider: "gcp"})
		} else if provider == "aws" {
			providers = append(providers, keys.Provider{GcpProject: "",
				Provider: "aws"})
		}
	}
	keySlice := keys.Keys(providers)
	keySlice = filterKeys(keySlice, spec)
	keySlice = adjustAges(keySlice, spec)
	if spec.RotationMode {
		//rotate keys
	} else {
		postMetric(keySlice, spec)
	}
}

//adjustAges returns a keys.Key slice containing the same keys but with keyAge
// changed to whatever's been configured (min/day/hour)
func adjustAges(keys []keys.Key, spec Specification) (adjustedKeys []keys.Key) {
	for _, key := range keys {
		key.Age = adjustAgeScale(key.Age, spec)
		adjustedKeys = append(adjustedKeys, key)
	}
	return adjustedKeys
}

//filterKeys returns a keys.Key slice created by filtering the provided
// keys.Key slice based on specific rules for each provider
func filterKeys(keys []keys.Key, spec Specification) (filteredKeys []keys.Key) {
	for _, key := range keys {
		valid := false
		switch key.Provider {
		case "gcp":
			valid = validGcpKey(key)
		case "aws":
			valid = validAwsKey(key, spec)
		}
		if valid {
			filteredKeys = append(filteredKeys, key)
		}
	}
	return
}

//validGcpKey returns a bool that reflects whether the provided keys.Key is
// valid, based on gcp-specific rules
func validGcpKey(key keys.Key) (valid bool) {
	// GCP managed keys should have roughly a week of life remaining.
	// User mnaaged keys by default have a lifetime of 10 years.
	// ...so this is a hack to split the GCP managed (which we don't care about)
	// from User managed keys (the latter should always pass on the first
	// condition)
	valid = math.Abs(key.LifeRemaining) > gcpAgeThresholdMins ||
		key.Age > gcpAgeThresholdMins
	return
}

//validAwsKey returns a bool that reflects whether the provided keys.Key is
// valid, based on aws-specific rules
func validAwsKey(key keys.Key, spec Specification) (valid bool) {
	if spec.IncludeAwsUserKeys {
		valid = true
	} else {
		match, _ := regexp.MatchString("[a-zA-Z]\\.[a-zA-Z]", key.Name)
		valid = !match
	}

	return
}

//adjustAgeScale converts the provided keyAge into the desired age
// granularity, based on the 'AgeMetricGranularity' in the provided
// Specification
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

//postMetric posts details of each keys.Key to a metrics api
func postMetric(keys []keys.Key, spec Specification) {
	url := strings.Join([]string{datadogURL, spec.DatadogAPIKey}, "")
	for _, key := range keys {
		var jsonString = []byte(
			`{ "series" :[{"metric":"key-dater.age","points":[[` +
				strconv.FormatInt(time.Now().Unix(), 10) +
				`, ` + strconv.FormatFloat(key.Age, 'f', 2, 64) +
				`]],"type":"count","tags":["team:cepheus","environment:prod","key:` +
				key.Name + `","id:` + key.ID + `","provider:` + key.Provider + `"]}]}`)
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

//check panics if error is not nil
func check(e error) {
	if e != nil {
		panic(e.Error())
	}
}
