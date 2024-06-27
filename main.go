package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	corev2 "github.com/sensu/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	Url    string
	Metric string
	Min    float64
	Max    float64
	Value  float64
}

type Tag struct {
	Name  model.LabelName
	Value model.LabelValue
}
type Metric struct {
	Tags  []Tag
	Value float64
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "sensu-prometheus-metrics-checks",
			Short:    "Check metrics from Prometheus",
			Keyspace: "sensu.io/plugins/sensu-prometheus-metrics-checks/config",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:     "url",
			Argument: "url",
			Default:  "http://localhost:9182/metrics",
			Usage:    "URL to the Prometheus metrics",
			Value:    &plugin.Url,
		},
		&sensu.PluginConfigOption[string]{
			Path:     "metric",
			Argument: "metric",
			Usage:    "Metric to check",
			Value:    &plugin.Metric,
		},
		&sensu.PluginConfigOption[float64]{
			Path:     "min",
			Argument: "min",
			Default:  math.Pi,
			Usage:    "Minimum value of metric",
			Value:    &plugin.Min,
		},
		&sensu.PluginConfigOption[float64]{
			Path:     "max",
			Argument: "max",
			Default:  math.Pi,
			Usage:    "Maximum value of metric",
			Value:    &plugin.Max,
		},
		&sensu.PluginConfigOption[float64]{
			Path:     "value",
			Argument: "value",
			Default:  math.Pi,
			Usage:    "Specific numeric value of metric",
			Value:    &plugin.Value,
		},
	}
)

func main() {
	check := sensu.NewCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(event *corev2.Event) (int, error) {
	if plugin.Metric == "" {
		return sensu.CheckStateUnknown, errors.New("--metric is required")
	}
	if plugin.Value == math.Pi && plugin.Max == math.Pi && plugin.Min == math.Pi {
		return sensu.CheckStateUnknown, errors.New("don't do that")
	}

	return sensu.CheckStateOK, nil
}
func QueryExporter(exporterURL string) (model.Vector, error) {

	tlsconfig := &tls.Config{InsecureSkipVerify: true}
	tr := &http.Transport{
		TLSClientConfig: tlsconfig,
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest("GET", exporterURL, nil)
	if err != nil {
		return nil, err
	}

	expResponse, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer expResponse.Body.Close()

	if expResponse.StatusCode != http.StatusOK {
		return nil, errors.New("exporter returned non OK HTTP response status: " + expResponse.Status)
	}

	var parser expfmt.TextParser

	metricFamilies, err := parser.TextToMetricFamilies(expResponse.Body)
	if err != nil {
		return nil, err
	}

	samples := model.Vector{}

	decodeOptions := &expfmt.DecodeOptions{
		Timestamp: model.Time(time.Now().Unix()),
	}

	for _, family := range metricFamilies {
		familySamples, _ := expfmt.ExtractSamples(decodeOptions, family)
		samples = append(samples, familySamples...)
	}

	return samples, nil
}
func executeCheck(event *corev2.Event) (int, error) {

	var samples model.Vector
	var err error

	samples, err = QueryExporter(plugin.Url)
	if err != nil {
		fmt.Printf("Failed: %s\n", err)
		return sensu.CheckStateUnknown, nil
	}

	for _, value := range samples {
		if value.Metric.String() == plugin.Metric {
			if plugin.Value != math.Pi && (value.Value != model.SampleValue(plugin.Value)) {
				fmt.Printf("Metric %s is at %f. Check require value %f\n", plugin.Metric, value.Value, plugin.Value)
				return sensu.CheckStateCritical, nil
			}
			if plugin.Min != math.Pi && (value.Value < model.SampleValue(plugin.Min)) {
				fmt.Printf("Metric %s is at %f. Check require minimum %f\n", plugin.Metric, value.Value, plugin.Min)
				return sensu.CheckStateCritical, nil
			}
			if plugin.Max != math.Pi && (value.Value > model.SampleValue(plugin.Max)) {
				fmt.Printf("Metric %s is at %f. Check require maximum %f\n", plugin.Metric, value.Value, plugin.Max)
				return sensu.CheckStateCritical, nil
			}
			fmt.Printf("Metric %s is at %f.\n", plugin.Metric, value.Value)
		}
	}
	return sensu.CheckStateOK, nil
}
