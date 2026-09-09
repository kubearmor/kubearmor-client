package install

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// GeneratePki - generate pub/priv keypair
func GeneratePki(namespace string, serviceName string) (*bytes.Buffer, *bytes.Buffer, *bytes.Buffer, error) {
	ca, cakey, err := GenerateCA()
	if err != nil {
		return bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{}), err
	}
	csr, csrkey, err := GenerateCSR(namespace, serviceName)
	if err != nil {
		return bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{}), err
	}
	crt, err := SignCSR(ca, cakey, csr, csrkey)
	if err != nil {
		return bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{}), err
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &cakey.PublicKey, cakey)
	if err != nil {
		return bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{}), err
	}
	caPEM := new(bytes.Buffer)
	err = pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	if err != nil {
		return bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{}), err
	}
	crtPEM := new(bytes.Buffer)
	err = pem.Encode(crtPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: crt,
	})
	if err != nil {
		return bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{}), err
	}
	crtKeyPEM := new(bytes.Buffer)
	err = pem.Encode(crtKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(csrkey),
	})
	if err != nil {
		return bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{}), err
	}
	return caPEM, crtPEM, crtKeyPEM, nil
}

// GenerateCA - generate private key and a cert for a CA
func GenerateCA() (*x509.Certificate, *rsa.PrivateKey, error) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(123),
		Subject: pkix.Name{
			Organization: []string{"kubearmor"},
			Country:      []string{"US"},
			Province:     []string{""},
			CommonName:   "kubearmor-ca",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(3, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCRLSign | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return &x509.Certificate{}, &rsa.PrivateKey{}, errors.New("cannot generate ca private key")
	}

	return ca, caPrivKey, nil
}

// GenerateCSR - generate certificate signing request
func GenerateCSR(namespace string, serviceName string) (*x509.Certificate, *rsa.PrivateKey, error) {
	csr := &x509.Certificate{
		SerialNumber: big.NewInt(1234),
		Subject: pkix.Name{
			Organization: []string{"kubearmor"},
			Country:      []string{"US"},
			Province:     []string{""},
			CommonName:   "kubearmor-webhook",
		},
		DNSNames: []string{
			serviceName + "." + namespace + ".svc",
			serviceName + "." + namespace + ".svc.cluster.local",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(3, 0, 0),
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		SubjectKeyId:          []byte{1, 2, 3, 4, 5},
		BasicConstraintsValid: true,
	}
	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return &x509.Certificate{}, &rsa.PrivateKey{}, errors.New("cannot generate csr private key")
	}
	return csr, certPrivKey, nil
}

// SignCSR - signs a certificate signing request essentially approving it using the given CA
func SignCSR(caCrt *x509.Certificate, caKey *rsa.PrivateKey, csrCrt *x509.Certificate, csrKey *rsa.PrivateKey) ([]byte, error) {
	certBytes, err := x509.CreateCertificate(rand.Reader, csrCrt, caCrt, &csrKey.PublicKey, caKey)
	if err != nil {
		return []byte{}, errors.New("cannot sign the csr")
	}
	return certBytes, nil
}

// Separate PKI for karmor unorchestrated (non-K8s) mode.
//
// The two trust domains are fully independent so that management and
// observability credentials are never interchangeable:
//
//	base/
//	  management/ca.crt, management/ca.key, management/client.crt, management/client.key
//	  log/ca.crt,        log/ca.key,        log/client.crt,        log/client.key
//
// No server.crt/server.key is generated or persisted: KubeArmor's
// SelfSignedCertLoader loads the respective CA and mints ephemeral server
// certificates in memory at startup. File names intentionally match the
// server-side layout (ca.crt/ca.key/client.crt/client.key, see
// KubeArmor/cert GetCACertPath/GetClientCertPath).
//
// Generation reuses the existing primitives above:
//   - GenerateCA() for the CA template + key (only the CommonName is
//     overridden per plane),
//   - SignCSR() for signing each plane's karmor client certificate,
//   - the same PEM encoding ("CERTIFICATE" / "RSA PRIVATE KEY").
//
// GenerateCSR()/GeneratePki() are deliberately NOT reused: they are wired
// for the K8s controller webhook (CN=kubearmor-webhook, webhook SANs) and
// GeneratePki() additionally drops the CA key, which the server needs on
// disk to mint its ephemeral server certificates.
const (
	// VMPlaneManagement is the management (policy/probe) trust domain.
	VMPlaneManagement = "management"
	// VMPlaneLog is the log/observability trust domain.
	VMPlaneLog = "log"

	// VMManagementCACommonName is the CommonName of the management CA.
	VMManagementCACommonName = "kubearmor-management-ca"
	// VMLogCACommonName is the CommonName of the log/observability CA.
	VMLogCACommonName = "kubearmor-log-ca"

	// VMManagementClientCommonName is the CommonName of the karmor management client cert.
	VMManagementClientCommonName = "karmor-management-client"
	// VMLogClientCommonName is the CommonName of the karmor log client cert.
	VMLogClientCommonName = "karmor-log-client"

	// DefaultVMTLSBaseDir is the default base directory holding the
	// management/ and log/ plane subdirectories.
	DefaultVMTLSBaseDir = "/var/lib/kubearmor/tls"

	caCertFile     = "ca.crt"
	caKeyFile      = "ca.key"
	clientCertFile = "client.crt"
	clientKeyFile  = "client.key"
)

// VMPlaneDir returns the directory holding a plane's PKI material.
func VMPlaneDir(baseDir, plane string) string {
	return filepath.Join(baseDir, plane)
}

// VMManagementDir returns the management plane directory.
func VMManagementDir(baseDir string) string {
	return VMPlaneDir(baseDir, VMPlaneManagement)
}

// VMLogDir returns the log/observability plane directory.
func VMLogDir(baseDir string) string {
	return VMPlaneDir(baseDir, VMPlaneLog)
}

// PlanePKI holds the PEM-encoded material for one trust plane.
type PlanePKI struct {
	CACert    *bytes.Buffer
	CAKey     *bytes.Buffer
	ClientCrt *bytes.Buffer
	ClientKey *bytes.Buffer
}

// GeneratePlanePKI generates a dedicated CA plus a karmor client
// certificate signed by that CA. It reuses GenerateCA() for the CA
// template/key and SignCSR() for the client certificate signature.
func GeneratePlanePKI(caCommonName, clientCommonName string) (*PlanePKI, error) {
	caTmpl, caKey, err := GenerateCA()
	if err != nil {
		return nil, err
	}
	// Only the identity is plane-specific; key type/size/validity and
	// key usages stay exactly as defined by GenerateCA().
	caTmpl.Subject.CommonName = caCommonName

	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("cannot self-sign the %s ca: %w", caCommonName, err)
	}
	caCertPEM := new(bytes.Buffer)
	if err := pem.Encode(caCertPEM, &pem.Block{Type: "CERTIFICATE", Bytes: caDER}); err != nil {
		return nil, err
	}
	// Unlike GeneratePki() (webhook flow keeps the CA key inside the
	// cluster signer), the unorchestrated server needs ca.key on disk:
	// SelfSignedCertLoader reads it to mint ephemeral server certs.
	caKeyPEM := new(bytes.Buffer)
	if err := pem.Encode(caKeyPEM, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(caKey)}); err != nil {
		return nil, err
	}

	clientTmpl := &x509.Certificate{
		SerialNumber: newClientSerial(),
		Subject: pkix.Name{
			Organization: []string{"kubearmor"},
			Country:      []string{"US"},
			CommonName:   clientCommonName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0), // valid for 1 year
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	clientKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("cannot generate %s private key: %w", clientCommonName, err)
	}
	clientDER, err := SignCSR(caTmpl, caKey, clientTmpl, clientKey)
	if err != nil {
		return nil, err
	}
	clientCrtPEM := new(bytes.Buffer)
	if err := pem.Encode(clientCrtPEM, &pem.Block{Type: "CERTIFICATE", Bytes: clientDER}); err != nil {
		return nil, err
	}
	clientKeyPEM := new(bytes.Buffer)
	if err := pem.Encode(clientKeyPEM, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)}); err != nil {
		return nil, err
	}

	return &PlanePKI{
		CACert:    caCertPEM,
		CAKey:     caKeyPEM,
		ClientCrt: clientCrtPEM,
		ClientKey: clientKeyPEM,
	}, nil
}

func newClientSerial() *big.Int {
	// 128-bit random serial (RFC 5280: positive, <= 20 octets). The legacy
	// webhook helpers use fixed serials; random serials are safe here and
	// avoid any cross-plane collision.
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return big.NewInt(1)
	}
	return serial
}

// planeComplete reports whether all four plane files already exist.
// server.crt/server.key are intentionally absent (see package doc).
func planeComplete(dir string) bool {
	for _, f := range []string{caCertFile, caKeyFile, clientCertFile, clientKeyFile} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			return false
		}
	}
	return true
}

// writePlaneFile writes data to dir/file with 0600 unless it already exists.
func writePlaneFile(dir, file string, data *bytes.Buffer) error {
	path := filepath.Join(dir, file)
	if _, err := os.Stat(path); err == nil {
		return nil // never clobber existing key material
	}
	return os.WriteFile(path, data.Bytes(), 0600)
}

// ensurePlane generates (via GeneratePlanePKI) and persists one plane,
// skipping planes whose four files already exist.
func ensurePlane(baseDir, plane, caCN, clientCN string) error {
	dir := VMPlaneDir(baseDir, plane)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	if planeComplete(dir) {
		return nil
	}
	pki, err := GeneratePlanePKI(caCN, clientCN)
	if err != nil {
		return err
	}
	if err := writePlaneFile(dir, caCertFile, pki.CACert); err != nil {
		return err
	}
	if err := writePlaneFile(dir, caKeyFile, pki.CAKey); err != nil {
		return err
	}
	if err := writePlaneFile(dir, clientCertFile, pki.ClientCrt); err != nil {
		return err
	}
	if err := writePlaneFile(dir, clientKeyFile, pki.ClientKey); err != nil {
		return err
	}
	return nil
}

// EnsureVMPlanePKI generates and persists both trust planes under baseDir.
// It is idempotent: planes whose ca.crt/ca.key/client.crt/client.key all
// exist are left untouched, so existing keys are never rotated by re-run.
func EnsureVMPlanePKI(baseDir string) error {
	if baseDir == "" {
		baseDir = DefaultVMTLSBaseDir
	}
	if err := ensurePlane(baseDir, VMPlaneManagement, VMManagementCACommonName, VMManagementClientCommonName); err != nil {
		return fmt.Errorf("cannot ensure management plane PKI: %w", err)
	}
	if err := ensurePlane(baseDir, VMPlaneLog, VMLogCACommonName, VMLogClientCommonName); err != nil {
		return fmt.Errorf("cannot ensure log plane PKI: %w", err)
	}
	return nil
}
