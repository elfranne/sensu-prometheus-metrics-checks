package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// testMetrics is a trimmed exposition-format body. node_filesystem_avail_bytes
// deliberately carries two series so the tests cover the fact that every sample
// of a metric is checked, not just the first one.
const testMetrics = `# HELP go_goroutines Number of goroutines that currently exist.
# TYPE go_goroutines gauge
go_goroutines 12
# HELP node_filesystem_avail_bytes Filesystem space available to non-root users in bytes.
# TYPE node_filesystem_avail_bytes gauge
node_filesystem_avail_bytes{device="/dev/sda1",fstype="ext4",mountpoint="/"} 1.073741824e+10
node_filesystem_avail_bytes{device="/dev/sda2",fstype="ext4",mountpoint="/boot"} 4.294967296e+09
# HELP up Exporter is up.
# TYPE up gauge
up 1
`

// exporter serves testMetrics over plain HTTP.
func exporter(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(testMetrics))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// resetPlugin puts the global plugin config back to the state the flag
// defaults in main.go would leave it in, pointed at url.
func resetPlugin(url string) {
	plugin.Url = url
	plugin.Metric = ""
	plugin.Min = math.Pi
	plugin.Max = math.Pi
	plugin.Value = math.Pi
	plugin.Labels = nil
	plugin.User = ""
	plugin.Password = ""
	plugin.Cert = ""
	plugin.Key = ""
	plugin.CaCert = ""
	plugin.insecureSkipVerify = false
}

// samplesFor returns the values of every sample named name.
func samplesFor(samples model.Vector, name string) []float64 {
	var values []float64
	for _, s := range samples {
		if s.Metric["__name__"] == model.LabelValue(name) {
			values = append(values, float64(s.Value))
		}
	}
	return values
}

func TestCheckArgs(t *testing.T) {
	tests := []struct {
		name    string
		metric  string
		min     float64
		max     float64
		value   float64
		want    int
		wantErr bool
	}{
		{name: "no metric", min: 1, want: sensu.CheckStateUnknown, wantErr: true},
		{name: "metric but no threshold", metric: "up", want: sensu.CheckStateUnknown, wantErr: true},
		{name: "metric and min", metric: "up", min: 1, want: sensu.CheckStateOK},
		{name: "metric and max", metric: "up", max: 1, want: sensu.CheckStateOK},
		{name: "metric and value", metric: "up", value: 1, want: sensu.CheckStateOK},
		{name: "metric and min and max", metric: "up", min: 0, max: 2, want: sensu.CheckStateOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetPlugin("")
			plugin.Metric = tt.metric
			if tt.min != 0 {
				plugin.Min = tt.min
			}
			if tt.max != 0 {
				plugin.Max = tt.max
			}
			if tt.value != 0 {
				plugin.Value = tt.value
			}
			// "metric and min and max" needs an explicit zero min.
			if tt.name == "metric and min and max" {
				plugin.Min = 0
			}

			status, err := checkArgs(nil)
			if status != tt.want {
				t.Errorf("status = %d, want %d", status, tt.want)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestQueryExporter(t *testing.T) {
	srv := exporter(t)

	samples, err := QueryExporter(srv.URL, "", "", false, "", "", "")
	if err != nil {
		t.Fatalf("QueryExporter() error = %v", err)
	}

	if got := samplesFor(samples, "go_goroutines"); len(got) != 1 || got[0] != 12 {
		t.Errorf("go_goroutines = %v, want [12]", got)
	}

	fs := samplesFor(samples, "node_filesystem_avail_bytes")
	if len(fs) != 2 {
		t.Errorf("node_filesystem_avail_bytes returned %d samples, want 2", len(fs))
	}

	// Labels must survive parsing; the --label assertion depends on them.
	var root *model.Sample
	for _, s := range samples {
		if s.Metric["__name__"] == "node_filesystem_avail_bytes" && s.Metric["mountpoint"] == "/" {
			root = s
			break
		}
	}
	if root == nil {
		t.Fatal("no node_filesystem_avail_bytes sample with mountpoint=/")
	}
	if root.Value != 1.073741824e+10 {
		t.Errorf("mountpoint=/ value = %v, want 1.073741824e+10", root.Value)
	}
	if root.Metric["fstype"] != "ext4" {
		t.Errorf("mountpoint=/ fstype = %q, want ext4", root.Metric["fstype"])
	}
}

func TestQueryExporterErrors(t *testing.T) {
	notFound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	t.Cleanup(notFound.Close)

	garbage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("this is not exposition format {{{\n"))
	}))
	t.Cleanup(garbage.Close)

	tests := []struct {
		name string
		url  string
	}{
		{name: "non-200 response", url: notFound.URL},
		{name: "unparseable body", url: garbage.URL},
		{name: "connection refused", url: "http://127.0.0.1:1/metrics"},
		{name: "malformed url", url: "://not a url"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := QueryExporter(tt.url, "", "", false, "", "", ""); err == nil {
				t.Error("QueryExporter() error = nil, want an error")
			}
		})
	}
}

func TestQueryExporterBasicAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "sensu" || password != "s3cret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(testMetrics))
	}))
	t.Cleanup(srv.Close)

	tests := []struct {
		name     string
		user     string
		password string
		wantErr  bool
	}{
		{name: "correct credentials", user: "sensu", password: "s3cret"},
		{name: "wrong password", user: "sensu", password: "wrong", wantErr: true},
		// Credentials are only sent when both halves are set.
		{name: "user only", user: "sensu", wantErr: true},
		{name: "password only", password: "s3cret", wantErr: true},
		{name: "no credentials", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := QueryExporter(srv.URL, tt.user, tt.password, false, "", "", "")
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestQueryExporterInsecureSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(testMetrics))
	}))
	t.Cleanup(srv.Close)

	if _, err := QueryExporter(srv.URL, "", "", false, "", "", ""); err == nil {
		t.Error("self-signed cert without --insecureskipverify: error = nil, want an error")
	}

	if _, err := QueryExporter(srv.URL, "", "", true, "", "", ""); err != nil {
		t.Errorf("self-signed cert with --insecureskipverify: error = %v", err)
	}
}

func TestQueryExporterMTLS(t *testing.T) {
	ca, caPool, serverCert, clientCert, clientKey := mtlsFixtures(t)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(testMetrics))
	}))
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    caPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	t.Run("full cert pair and ca", func(t *testing.T) {
		if _, err := QueryExporter(srv.URL, "", "", false, clientCert, clientKey, ca); err != nil {
			t.Errorf("QueryExporter() error = %v", err)
		}
	})

	// The mTLS branch is entered as soon as any one of the three is set, so a
	// partial configuration fails rather than silently falling back.
	t.Run("ca only", func(t *testing.T) {
		if _, err := QueryExporter(srv.URL, "", "", false, "", "", ca); err == nil {
			t.Error("error = nil, want an error")
		}
	})

	t.Run("client pair without ca", func(t *testing.T) {
		if _, err := QueryExporter(srv.URL, "", "", false, clientCert, clientKey, ""); err == nil {
			t.Error("error = nil, want an error")
		}
	})

	// insecureSkipVerify does not rescue a request with no client certificate:
	// the server rejects the handshake.
	t.Run("no client cert", func(t *testing.T) {
		if _, err := QueryExporter(srv.URL, "", "", true, "", "", ""); err == nil {
			t.Error("error = nil, want an error")
		}
	})
}

func TestExecuteCheck(t *testing.T) {
	srv := exporter(t)

	tests := []struct {
		name   string
		setup  func()
		want   int
		urlFor string
	}{
		{
			name:  "value within min",
			setup: func() { plugin.Metric = "go_goroutines"; plugin.Min = 1 },
			want:  sensu.CheckStateOK,
		},
		{
			name:  "value below min",
			setup: func() { plugin.Metric = "go_goroutines"; plugin.Min = 100 },
			want:  sensu.CheckStateCritical,
		},
		{
			name:  "value above max",
			setup: func() { plugin.Metric = "go_goroutines"; plugin.Max = 1 },
			want:  sensu.CheckStateCritical,
		},
		{
			name:  "value within min and max",
			setup: func() { plugin.Metric = "go_goroutines"; plugin.Min = 1; plugin.Max = 100 },
			want:  sensu.CheckStateOK,
		},
		{
			name:  "exact value matches",
			setup: func() { plugin.Metric = "up"; plugin.Value = 1 },
			want:  sensu.CheckStateOK,
		},
		{
			name:  "exact value differs",
			setup: func() { plugin.Metric = "up"; plugin.Value = 0 },
			want:  sensu.CheckStateCritical,
		},
		{
			// Both series are checked, and /boot is under the threshold.
			name:  "one of two series below min",
			setup: func() { plugin.Metric = "node_filesystem_avail_bytes"; plugin.Min = 5e+09 },
			want:  sensu.CheckStateCritical,
		},
		{
			name:  "all series above min",
			setup: func() { plugin.Metric = "node_filesystem_avail_bytes"; plugin.Min = 1e+09 },
			want:  sensu.CheckStateOK,
		},
		{
			// --label is an assertion: the /boot series does not carry
			// mountpoint=/, so it fails even though its value is fine.
			name: "label not matched by every series",
			setup: func() {
				plugin.Metric = "node_filesystem_avail_bytes"
				plugin.Labels = []string{"mountpoint:/"}
				plugin.Min = 1e+09
			},
			want: sensu.CheckStateCritical,
		},
		{
			name: "label matched by every series",
			setup: func() {
				plugin.Metric = "node_filesystem_avail_bytes"
				plugin.Labels = []string{"fstype:ext4"}
				plugin.Min = 1e+09
			},
			want: sensu.CheckStateOK,
		},
		{
			name: "label values are trimmed",
			setup: func() {
				plugin.Metric = "node_filesystem_avail_bytes"
				plugin.Labels = []string{" fstype : ext4 "}
				plugin.Min = 1e+09
			},
			want: sensu.CheckStateOK,
		},
		{
			name:  "metric absent from exporter",
			setup: func() { plugin.Metric = "does_not_exist"; plugin.Min = 1 },
			want:  sensu.CheckStateUnknown,
		},
		{
			name:   "exporter unreachable",
			setup:  func() { plugin.Metric = "go_goroutines"; plugin.Min = 1 },
			urlFor: "http://127.0.0.1:1/metrics",
			want:   sensu.CheckStateUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := srv.URL
			if tt.urlFor != "" {
				url = tt.urlFor
			}
			resetPlugin(url)
			tt.setup()

			status, err := executeCheck(nil)
			if err != nil {
				t.Fatalf("executeCheck() error = %v", err)
			}
			if status != tt.want {
				t.Errorf("status = %d, want %d", status, tt.want)
			}
		})
	}
}

// mtlsFixtures generates a throwaway CA, a server certificate for 127.0.0.1 and
// a client certificate, and writes the CA and client pair to a temp dir. It
// returns the paths of the CA, client cert and client key files, plus the
// in-memory server certificate and CA pool for the test server.
func mtlsFixtures(t *testing.T) (caFile string, caPool *x509.CertPool, serverCert tls.Certificate, certFile, keyFile string) {
	t.Helper()
	dir := t.TempDir()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse CA cert: %v", err)
	}

	caFile = filepath.Join(dir, "ca.pem")
	writePEM(t, caFile, "CERTIFICATE", caDER)

	caPool = x509.NewCertPool()
	caPool.AddCert(caCert)

	serverCert = issue(t, dir, "server", caCert, caKey, x509.ExtKeyUsageServerAuth)
	_ = issue(t, dir, "client", caCert, caKey, x509.ExtKeyUsageClientAuth)

	return caFile, caPool, serverCert, filepath.Join(dir, "client.pem"), filepath.Join(dir, "client-key.pem")
}

// issue signs a leaf certificate for name, writes it to dir as name.pem /
// name-key.pem, and returns it loaded as a tls.Certificate.
func issue(t *testing.T, dir, name string, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, usage x509.ExtKeyUsage) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate %s key: %v", name, err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.IPv6loopback},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create %s cert: %v", name, err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal %s key: %v", name, err)
	}

	certPath := filepath.Join(dir, name+".pem")
	keyPath := filepath.Join(dir, name+"-key.pem")
	writePEM(t, certPath, "CERTIFICATE", der)
	writePEM(t, keyPath, "EC PRIVATE KEY", keyDER)

	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Fatalf("load %s key pair: %v", name, err)
	}
	return pair
}

func writePEM(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
