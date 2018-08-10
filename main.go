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
	DatadogAPIKey        string `required:"true"`
}

func main() {
	var spec Specification
	err := envconfig.Process(envConfigPrefix, &spec)
	check(err)
	providers := make([]keys.Provider, 0)
	providers = append(providers, keys.Provider{GcpProject: spec.GcpProject,
		Provider: "gcp"})
	providers = append(providers, keys.Provider{GcpProject: "",
		Provider: "aws"})
	keySlice := keys.Keys(providers)
	keySlice = filterKeys(keySlice)
	postMetric(keySlice, spec)
}

func filterKeys(keys []keys.Key) (filteredKeys []keys.Key) {
	for _, key := range keys {
		valid := false
		switch key.Provider {
		case "gcp":
			valid = validGcpKey(key)
		case "aws":
			valid = validAwsKey(key)
		}
		if valid {
			filteredKeys = append(filteredKeys, key)
		}
	}
	return
}

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

func validAwsKey(key keys.Key) (valid bool) {
	match, _ := regexp.MatchString("^[a-zA-Z]+$\\.^[a-zA-Z]+$", key.Name)
	valid = !match
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

func postMetric(keys []keys.Key, spec Specification) {
	url := strings.Join([]string{datadogURL, spec.DatadogAPIKey}, "")
	for _, key := range keys {
		var jsonString = []byte(
			`{ "series" :[{"metric":"key-dater.age","points":[[` +
				strconv.FormatInt(time.Now().Unix(), 10) +
				`, ` + strconv.FormatFloat(key.Age, 'f', 2, 64) +
				`]],"type":"count","tags":["team:cepheus","environment:prod","key:` +
				key.Name + `","id:` + key.ID + `"]}]}`)
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
