[![Sensu Bonsai Asset](https://img.shields.io/badge/Bonsai-Download%20Me-brightgreen.svg?colorB=89C967&logo=sensu)](https://bonsai.sensu.io/assets/elfranne/sensu-prometheus-metrics-checks)
![Go Test](https://github.com/elfranne/sensu-prometheus-metrics-checks/workflows/Go%20Test/badge.svg)
![goreleaser](https://github.com/elfranne/sensu-prometheus-metrics-checks/workflows/goreleaser/badge.svg)

# sensu-prometheus-metrics-checks

## Table of Contents
- [Overview](#overview)
- [Usage](#usage)
- [Usage examples](#usage-examples)
- [Configuration](#configuration)
  - [Asset registration](#asset-registration)
  - [Check definition](#check-definition)
- [Installation from source](#installation-from-source)
- [Releases](#releases)
- [Contributing](#contributing)

## Overview

sensu-prometheus-metrics-checks is a [Sensu Check][6] that scrapes a Prometheus exporter
endpoint, picks out a single metric by name, and asserts that its value is within the
bounds you configure.

It reads the exporter's [text exposition format][11] response, selects every sample whose
`__name__` equals `--metric`, and compares each one against `--min`, `--max` and
`--value`. Any sample outside those bounds — or missing one of the labels given with
`--label` — makes the check exit **CRITICAL**. The check exits **UNKNOWN** if the exporter
cannot be reached, returns a non-200 response, or does not expose the requested metric at
all. Otherwise it exits **OK**.

The exporter can be behind HTTP basic auth (`--user` / `--password`) or mutual TLS
(`--cert` / `--key` / `--cacert`).

## Usage

```
Check metrics from Prometheus

Usage:
  sensu-prometheus-metrics-checks [flags]
  sensu-prometheus-metrics-checks [command]
```

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `--metric` | *(none, required)* | Name of the metric to check. |
| `--url` | `http://localhost:9182/metrics` | URL of the Prometheus metrics endpoint. |
| `--min` | *(unset)* | Fail if the metric is below this value. |
| `--max` | *(unset)* | Fail if the metric is above this value. |
| `--value` | *(unset)* | Fail if the metric is not exactly this value. |
| `--label` | *(none)* | Require the metric to carry the label `name:value`. Repeatable. |
| `--user` | *(none)* | Username for HTTP basic auth. |
| `--password` | *(none)* | Password for HTTP basic auth. |
| `--cert` | *(none)* | Client certificate to use for mTLS. |
| `--key` | *(none)* | Client key to use for mTLS. |
| `--cacert` | *(none)* | CA certificate to use for mTLS. |
| `--insecureskipverify` | `false` | Skip TLS certificate verification (self-signed certs). |

Notes:

- `--metric` is mandatory, and **at least one** of `--min`, `--max` or `--value` must be
  given. Running with none of them exits UNKNOWN.
- `--min`, `--max` and `--value` can be combined; each is checked independently.
- `--label` takes `name:value`, split on the first `:`. It is an assertion, not a filter:
  every sample of `--metric` must carry all the labels you list, otherwise the check is
  CRITICAL. Use it when the metric has exactly one series you care about.
- Basic auth is only sent when both `--user` and `--password` are set.
- mTLS needs `--cert`, `--key` and `--cacert` together; setting only some of them makes
  the check fail to load the key pair. When mTLS is configured, `--insecureskipverify` is
  ignored.
- Every flag can also be set through a Sensu entity or check annotation under the keyspace
  `sensu.io/plugins/sensu-prometheus-metrics-checks/config`, for example
  `sensu.io/plugins/sensu-prometheus-metrics-checks/config/url`.

## Usage examples

Alert when the root filesystem reported by node_exporter drops below 5 GiB:

```
sensu-prometheus-metrics-checks \
  --url http://localhost:9100/metrics \
  --metric node_filesystem_avail_bytes \
  --label 'mountpoint:/' \
  --label 'fstype:ext4' \
  --min 5368709120
```

Alert when a systemd unit is not active (`node_systemd_unit_state` is `1` for the active
state):

```
sensu-prometheus-metrics-checks \
  --url http://localhost:9100/metrics \
  --metric node_systemd_unit_state \
  --label 'name:sshd.service' \
  --label 'state:active' \
  --value 1
```

Scrape an exporter that requires mutual TLS:

```
sensu-prometheus-metrics-checks \
  --url https://exporter.example.com:9100/metrics \
  --metric go_goroutines \
  --max 1000 \
  --cert /etc/sensu/certs/client.crt \
  --key /etc/sensu/certs/client.key \
  --cacert /etc/sensu/certs/ca.crt
```

## Configuration

### Asset registration

[Sensu Assets][10] are the best way to make use of this plugin. If you're not using an asset, please
consider doing so! If you're using sensuctl 5.13 with Sensu Backend 5.13 or later, you can use the
following command to add the asset:

```
sensuctl asset add elfranne/sensu-prometheus-metrics-checks
```

If you're using an earlier version of sensuctl, you can find the asset on the
[Bonsai Asset Index](https://bonsai.sensu.io/assets/elfranne/sensu-prometheus-metrics-checks).

### Check definition

```yml
---
type: CheckConfig
api_version: core/v2
metadata:
  name: node-root-filesystem-free
  namespace: default
spec:
  command: >-
    sensu-prometheus-metrics-checks
    --url http://localhost:9100/metrics
    --metric node_filesystem_avail_bytes
    --label 'mountpoint:/'
    --label 'fstype:ext4'
    --min 5368709120
  subscriptions:
  - system
  runtime_assets:
  - elfranne/sensu-prometheus-metrics-checks
```

## Installation from source

The preferred way of installing and deploying this plugin is to use it as an Asset. If you would
like to compile and install the plugin from source or contribute to it, download the latest version
or create an executable script from this source.

From the local path of the sensu-prometheus-metrics-checks repository:

```
go build
```

## Releases

Tag the target sha with a semver release without a `v` prefix (ex. `1.0.0`). This triggers
the [GitHub action][5] workflow that [builds and releases][4] the plugin with goreleaser.

## Contributing

For more information about contributing to this plugin, see [Contributing][1].

[1]: https://github.com/sensu/sensu-go/blob/master/CONTRIBUTING.md
[4]: https://github.com/elfranne/sensu-prometheus-metrics-checks/blob/master/.github/workflows/release.yml
[5]: https://github.com/elfranne/sensu-prometheus-metrics-checks/actions
[6]: https://docs.sensu.io/sensu-go/latest/reference/checks/
[10]: https://docs.sensu.io/sensu-go/latest/reference/assets/
[11]: https://prometheus.io/docs/instrumenting/exposition_formats/
