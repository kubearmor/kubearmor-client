package install

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/Masterminds/sprig"
	"sigs.k8s.io/yaml"
)

func parseTestCert(t *testing.T, pemBytes []byte) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		t.Fatal("failed to decode PEM block")
	}
	crt, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}
	return crt
}

func TestGeneratePlanePKI_CommonNames(t *testing.T) {
	mgmt, err := GeneratePlanePKI(VMManagementCACommonName, VMManagementClientCommonName)
	if err != nil {
		t.Fatalf("GeneratePlanePKI management failed: %v", err)
	}
	logPKI, err := GeneratePlanePKI(VMLogCACommonName, VMLogClientCommonName)
	if err != nil {
		t.Fatalf("GeneratePlanePKI log failed: %v", err)
	}

	mgmtCA := parseTestCert(t, mgmt.CACert.Bytes())
	if mgmtCA.Subject.CommonName != VMManagementCACommonName {
		t.Errorf("management CA CN = %q, want %q", mgmtCA.Subject.CommonName, VMManagementCACommonName)
	}
	if !mgmtCA.IsCA {
		t.Error("management CA IsCA = false, want true")
	}
	logCA := parseTestCert(t, logPKI.CACert.Bytes())
	if logCA.Subject.CommonName != VMLogCACommonName {
		t.Errorf("log CA CN = %q, want %q", logCA.Subject.CommonName, VMLogCACommonName)
	}

	mgmtClient := parseTestCert(t, mgmt.ClientCrt.Bytes())
	if mgmtClient.Subject.CommonName != VMManagementClientCommonName {
		t.Errorf("management client CN = %q, want %q", mgmtClient.Subject.CommonName, VMManagementClientCommonName)
	}
	if mgmtClient.IsCA {
		t.Error("management client IsCA = true, want false")
	}
	hasClientAuth := false
	for _, eku := range mgmtClient.ExtKeyUsage {
		if eku == x509.ExtKeyUsageClientAuth {
			hasClientAuth = true
		}
		if eku == x509.ExtKeyUsageServerAuth {
			t.Error("client cert must not have ServerAuth EKU")
		}
	}
	if !hasClientAuth {
		t.Error("client cert missing ClientAuth EKU")
	}
}

func TestGeneratePlanePKI_CAsAreDistinct(t *testing.T) {
	mgmt, err := GeneratePlanePKI(VMManagementCACommonName, VMManagementClientCommonName)
	if err != nil {
		t.Fatal(err)
	}
	logPKI, err := GeneratePlanePKI(VMLogCACommonName, VMLogClientCommonName)
	if err != nil {
		t.Fatal(err)
	}
	mgmtCA := parseTestCert(t, mgmt.CACert.Bytes())
	logCA := parseTestCert(t, logPKI.CACert.Bytes())
	if string(mgmtCA.RawSubject) == string(logCA.RawSubject) {
		t.Error("management and log CAs share the same subject, want distinct trust roots")
	}
	mgmtPub, err := x509.MarshalPKIXPublicKey(mgmtCA.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	logPub, err := x509.MarshalPKIXPublicKey(logCA.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if string(mgmtPub) == string(logPub) {
		t.Error("management and log CAs share the same public key, want distinct trust roots")
	}
}

func verifyClientAgainstCA(t *testing.T, clientPEM, caPEM []byte) error {
	t.Helper()
	client := parseTestCert(t, clientPEM)
	ca := parseTestCert(t, caPEM)
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	_, err := client.Verify(x509.VerifyOptions{Roots: pool, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}})
	return err
}

func TestGeneratePlanePKI_ClientsAreNotInterchangeable(t *testing.T) {
	mgmt, err := GeneratePlanePKI(VMManagementCACommonName, VMManagementClientCommonName)
	if err != nil {
		t.Fatal(err)
	}
	logPKI, err := GeneratePlanePKI(VMLogCACommonName, VMLogClientCommonName)
	if err != nil {
		t.Fatal(err)
	}
	// Each client verifies against its own CA.
	if err := verifyClientAgainstCA(t, mgmt.ClientCrt.Bytes(), mgmt.CACert.Bytes()); err != nil {
		t.Errorf("management client does not verify against management CA: %v", err)
	}
	if err := verifyClientAgainstCA(t, logPKI.ClientCrt.Bytes(), logPKI.CACert.Bytes()); err != nil {
		t.Errorf("log client does not verify against log CA: %v", err)
	}
	// Cross-plane verification must fail.
	if err := verifyClientAgainstCA(t, mgmt.ClientCrt.Bytes(), logPKI.CACert.Bytes()); err == nil {
		t.Error("management client verified against log CA, want failure (planes must not be interchangeable)")
	}
	if err := verifyClientAgainstCA(t, logPKI.ClientCrt.Bytes(), mgmt.CACert.Bytes()); err == nil {
		t.Error("log client verified against management CA, want failure (planes must not be interchangeable)")
	}
}

func TestSetupVMTLS_CreatesBothPlanes(t *testing.T) {
	base := t.TempDir()
	if err := SetupVMTLS(base); err != nil {
		t.Fatalf("SetupVMTLS failed: %v", err)
	}
	for _, plane := range []string{VMPlaneManagement, VMPlaneLog} {
		for _, f := range []string{caCertFile, caKeyFile, clientCertFile, clientKeyFile} {
			if _, err := os.Stat(filepath.Join(VMPlaneDir(base, plane), f)); err != nil {
				t.Errorf("plane %s: expected file %s: %v", plane, f, err)
			}
		}
	}
	// Re-run must preserve existing key material (idempotent install).
	snap, err := os.ReadFile(filepath.Join(VMManagementDir(base), caCertFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := SetupVMTLS(base); err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(filepath.Join(VMManagementDir(base), caCertFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(snap) != string(again) {
		t.Error("SetupVMTLS re-run rotated the management CA, want existing key material preserved")
	}
}

// The agent must be pointed at the split-plane PKI: the systemd unit and
// the docker-compose template pass both TLS flags, the agent config
// template carries both TLS keys, and the compose template mounts the TLS
// base directory into the agent container.
func TestUnorchestratedTemplatesCarrySplitPlaneTLS(t *testing.T) {
	for _, flag := range []string{
		"-tlsCertPath=/var/lib/kubearmor/tls/log",
		"-managementTLSCertPath=/var/lib/kubearmor/tls/management",
	} {
		if !strings.Contains(kubearmorServiceFile, flag) {
			t.Errorf("kubearmor.service missing %q", flag)
		}
		if !strings.Contains(kubearmorcomposeTemplate, flag) {
			t.Errorf("compose template missing %q", flag)
		}
	}
	for _, key := range []string{
		"tlsCertPath: /var/lib/kubearmor/tls/log",
		"managementTLSCertPath: /var/lib/kubearmor/tls/management",
	} {
		if !strings.Contains(kubeArmorConfig, key) {
			t.Errorf("config template missing %q", key)
		}
	}
	if !strings.Contains(kubearmorcomposeTemplate, "/var/lib/kubearmor/tls:/var/lib/kubearmor/tls:ro") {
		t.Error("compose template missing read-only TLS volume mount")
	}
}

func sampleUnorchestratedConfig() *KubeArmorConfig {
	return &KubeArmorConfig{
		Hostname:                    "test-host",
		KubeArmorImage:              "kubearmor/kubearmor:test",
		KubeArmorInitImage:          "kubearmor/kubearmor-init:test",
		ImagePullPolicy:             "always",
		KubeArmorVisibility:         "process,network",
		KubeArmorHostVisibility:     "process,network",
		KubeArmorFilePosture:        "block",
		KubeArmorNetworkPosture:     "block",
		KubeArmorCapPosture:         "block",
		KubeArmorHostFilePosture:    "block",
		KubeArmorHostNetworkPosture: "block",
		KubeArmorHostCapPosture:     "block",
		KubeArmorAlertThrottling:    true,
		KubeArmorMaxAlertsPerSec:    10,
		KubeArmorThrottleSec:        30,
		KubeArmorPort:               "32767",
		SecureContainers:            false,
	}
}

func renderTemplate(t *testing.T, name, tmpl string, data *KubeArmorConfig) map[string]interface{} {
	t.Helper()
	var buf bytes.Buffer
	tpl, err := template.New(name).Funcs(sprig.GenericFuncMap()).Parse(tmpl)
	if err != nil {
		t.Fatalf("template %s does not parse: %v", name, err)
	}
	if err := tpl.Execute(&buf, data); err != nil {
		t.Fatalf("template %s does not render: %v", name, err)
	}
	var out map[string]interface{}
	if err := yaml.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("rendered %s is not valid YAML: %v", name, err)
	}
	return out
}

func strSlice(t *testing.T, v interface{}) []string {
	t.Helper()
	raw, ok := v.([]interface{})
	if !ok {
		t.Fatalf("expected a YAML list, got %T", v)
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("expected list of strings, got %T", item)
		}
		out = append(out, s)
	}
	return out
}

// Render-level smoke test for the agent side: the compose and agent-config
// templates must render to valid YAML carrying the split-plane TLS wiring
// the agent needs to serve mTLS from the generated PKI layout.
func TestUnorchestratedTemplatesRenderSplitPlaneTLS(t *testing.T) {
	cfg := sampleUnorchestratedConfig()

	compose := renderTemplate(t, "compose", kubearmorcomposeTemplate, cfg)
	svcs, ok := compose["services"].(map[string]interface{})
	if !ok {
		t.Fatal("rendered compose has no services map")
	}
	agent, ok := svcs["kubearmor"].(map[string]interface{})
	if !ok {
		t.Fatal("rendered compose has no kubearmor service")
	}
	cmd := strSlice(t, agent["command"])
	for _, want := range []string{
		"-tlsCertPath=/var/lib/kubearmor/tls/log",
		"-managementTLSCertPath=/var/lib/kubearmor/tls/management",
	} {
		found := false
		for _, arg := range cmd {
			if arg == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("rendered compose command missing %q (got %v)", want, cmd)
		}
	}
	vols := strSlice(t, agent["volumes"])
	found := false
	for _, v := range vols {
		if v == "/var/lib/kubearmor/tls:/var/lib/kubearmor/tls:ro" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("rendered compose volumes missing read-only TLS mount (got %v)", vols)
	}

	agentCfg := renderTemplate(t, "kubearmor-config", kubeArmorConfig, cfg)
	if agentCfg["tlsCertPath"] != "/var/lib/kubearmor/tls/log" {
		t.Errorf("rendered agent config tlsCertPath = %v", agentCfg["tlsCertPath"])
	}
	if agentCfg["managementTLSCertPath"] != "/var/lib/kubearmor/tls/management" {
		t.Errorf("rendered agent config managementTLSCertPath = %v", agentCfg["managementTLSCertPath"])
	}
}

func TestEnsureVMPlanePKI_Layout(t *testing.T) {
	base := t.TempDir()
	if err := EnsureVMPlanePKI(base); err != nil {
		t.Fatalf("EnsureVMPlanePKI failed: %v", err)
	}
	for _, plane := range []string{VMPlaneManagement, VMPlaneLog} {
		dir := VMPlaneDir(base, plane)
		for _, f := range []string{caCertFile, caKeyFile, clientCertFile, clientKeyFile} {
			info, err := os.Stat(filepath.Join(dir, f))
			if err != nil {
				t.Errorf("plane %s: expected file %s: %v", plane, f, err)
				continue
			}
			if info.Mode().Perm() != 0600 {
				t.Errorf("plane %s file %s perm = %o, want 600", plane, f, info.Mode().Perm())
			}
			if info.Size() == 0 {
				t.Errorf("plane %s file %s is empty", plane, f)
			}
		}
		// No persisted server key material.
		for _, f := range []string{"server.crt", "server.key"} {
			if _, err := os.Stat(filepath.Join(dir, f)); !os.IsNotExist(err) {
				t.Errorf("plane %s: %s must not be persisted", plane, f)
			}
		}
	}
}

func TestEnsureVMPlanePKI_Idempotent(t *testing.T) {
	base := t.TempDir()
	if err := EnsureVMPlanePKI(base); err != nil {
		t.Fatal(err)
	}
	before := map[string][]byte{}
	for _, plane := range []string{VMPlaneManagement, VMPlaneLog} {
		for _, f := range []string{caCertFile, caKeyFile, clientCertFile, clientKeyFile} {
			b, err := os.ReadFile(filepath.Join(VMPlaneDir(base, plane), f))
			if err != nil {
				t.Fatal(err)
			}
			before[plane+"/"+f] = b
		}
	}
	if err := EnsureVMPlanePKI(base); err != nil {
		t.Fatal(err)
	}
	for k, b := range before {
		after, err := os.ReadFile(filepath.Join(base, k))
		if err != nil {
			t.Fatal(err)
		}
		if string(after) != string(b) {
			t.Errorf("re-run rotated %s, want existing key material preserved", k)
		}
	}
}

func TestEnsureVMPlanePKI_OnDiskClientsVerify(t *testing.T) {
	base := t.TempDir()
	if err := EnsureVMPlanePKI(base); err != nil {
		t.Fatal(err)
	}
	read := func(plane, f string) []byte {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(VMPlaneDir(base, plane), f))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	mgmtCrt, mgmtCA := read(VMPlaneManagement, clientCertFile), read(VMPlaneManagement, caCertFile)
	logCrt, logCA := read(VMPlaneLog, clientCertFile), read(VMPlaneLog, caCertFile)
	if err := verifyClientAgainstCA(t, mgmtCrt, mgmtCA); err != nil {
		t.Errorf("on-disk management client does not verify: %v", err)
	}
	if err := verifyClientAgainstCA(t, logCrt, logCA); err != nil {
		t.Errorf("on-disk log client does not verify: %v", err)
	}
	if err := verifyClientAgainstCA(t, mgmtCrt, logCA); err == nil {
		t.Error("on-disk management client verified against log CA, want failure")
	}
}
