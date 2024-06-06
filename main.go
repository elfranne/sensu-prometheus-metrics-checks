package main

import (
	"fmt"
	"io"
	"math"
	"net/http"

	corev2 "github.com/sensu/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	Min    float32
	Max    float32
	Value  float32
	String string
	Url    string
	Metric string
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-process-ressources",
			Short:    "Check if process is using too much ressources (CPU/memory)",
			Keyspace: "sensu.io/plugins/check-process-ressources/config",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[float32]{
			Path:     "min",
			Argument: "min",
			Usage:    "Minimum value of metric",
			Value:    &plugin.Min,
		},
		&sensu.PluginConfigOption[float32]{
			Path:     "max",
			Argument: "max",
			Usage:    "maximum value of metric",
			Value:    &plugin.Max,
		},
		&sensu.PluginConfigOption[float32]{
			Path:     "value",
			Argument: "value",
			Usage:    "Specific numeric value of metric",
			Value:    &plugin.Value,
		},
		&sensu.PluginConfigOption[string]{
			Path:     "string",
			Argument: "string",
			Usage:    "Specific string value of metric",
			Value:    &plugin.String,
		},
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
	}
)

func main() {
	check := sensu.NewCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(event *corev2.Event) (int, error) {
	return sensu.CheckStateOK, nil
}

func Round(x, unit float64) float64 {
	return math.Round(x/unit) * unit
}

func executeCheck(event *corev2.Event) (int, error) {
	resp, err := http.Get(plugin.Url)
	if err != nil {
		fmt.Printf("failed to query metrics: %s\n", err)
		return sensu.CheckStateUnknown, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("failed to parse body: %s\n", err)
		return sensu.CheckStateUnknown, nil
	}

	// fmt.Printf("body: %s\n", body)
	for _, m := range string(body) {

		fmt.Printf("checking metric: %s\n", string(m))

	}
	// 	v := strings.Fields(string(m))
	// 	if v[0] == plugin.Metric {
	// 		fmt.Printf("checking metric: %s\n", v[0])

	// if len(plugin.String) > 0 && v[1] != plugin.String {
	// 	fmt.Printf("metric %s is not matching %s\n", v[0], v[1])
	// 	return sensu.CheckStateCritical, nil
	// }

	// if float32(v[1]) {

	// }

	// if plugin.Max && v[1] > plugin.Max {
	// 	fmt.Printf("metric %s is exeedind max (%f): %f\n", v[0], plugin.Max, v[1])
	// 	return sensu.CheckStateCritical, nil
	// }
	// if len(plugin.Min) > 0 && v[1] < plugin.Min {
	// 	fmt.Printf("metric %s is lower than min (%f): %f\n", v[0], plugin.Min, v[1])
	// 	return sensu.CheckStateCritical, nil
	// }
	// if len(plugin.Value) > 0 && v[1] == plugin.Value {
	// 	fmt.Printf("metric %s is not equal (%f): %f\n", v[0], plugin.Value, v[1])
	// 	return sensu.CheckStateCritical, nil
	// }
	// 	}
	// }
	return sensu.CheckStateOK, nil
}
