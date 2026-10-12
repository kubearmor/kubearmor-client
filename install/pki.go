package install

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
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

func readPlaneFile(dir *os.Root, name string) ([]byte, error) {
	info, err := dir.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s must be a regular file, not a symlink", name)
	}
	if info.Mode().Perm()&0022 != 0 || ((name == caKeyFile || name == clientKeyFile) && info.Mode().Perm()&0077 != 0) {
		return nil, fmt.Errorf("%s has unsafe permissions %o", name, info.Mode().Perm())
	}
	f, err := dir.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, opened) {
		return nil, fmt.Errorf("%s changed while reading PKI", name)
	}
	return io.ReadAll(f)
}

func parsePlaneCert(data []byte) (*x509.Certificate, error) {
	block, rest := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" || len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("expected one PEM certificate")
	}
	return x509.ParseCertificate(block.Bytes)
}

func validatePlane(files map[string][]byte) error {
	ca, err := parsePlaneCert(files[caCertFile])
	if err != nil {
		return fmt.Errorf("invalid %s: %w", caCertFile, err)
	}
	if !ca.IsCA || !ca.BasicConstraintsValid {
		return fmt.Errorf("%s must be a CA certificate", caCertFile)
	}
	// The agent's self provider requires an RSA key in PKCS#1 format.
	block, rest := pem.Decode(files[caKeyFile])
	if block == nil || block.Type != "RSA PRIVATE KEY" || len(bytes.TrimSpace(rest)) != 0 {
		return fmt.Errorf("%s must contain one PKCS#1 RSA private key", caKeyFile)
	}
	caKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("invalid %s: %w", caKeyFile, err)
	}
	if err := caKey.Validate(); err != nil {
		return fmt.Errorf("invalid %s: %w", caKeyFile, err)
	}
	if !caKey.PublicKey.Equal(ca.PublicKey) {
		return fmt.Errorf("%s does not match %s", caKeyFile, caCertFile)
	}
	client, err := parsePlaneCert(files[clientCertFile])
	if err != nil {
		return fmt.Errorf("invalid %s: %w", clientCertFile, err)
	}
	if _, err := tls.X509KeyPair(files[clientCertFile], files[clientKeyFile]); err != nil {
		return fmt.Errorf("invalid client certificate/key pair: %w", err)
	}
	if client.IsCA || bytes.Equal(ca.RawSubjectPublicKeyInfo, client.RawSubjectPublicKeyInfo) {
		return errors.New("client certificate must use a separate, non-CA identity")
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	if _, err := client.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		return fmt.Errorf("client certificate is not currently valid for this CA: %w", err)
	}
	return nil
}

// existingPlane returns false only for a missing or empty directory. Any
// incomplete or invalid existing identity must be repaired explicitly.
func existingPlane(base *os.Root, plane string) (bool, error) {
	info, err := base.Lstat(plane)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() || info.Mode().Perm()&0022 != 0 {
		return false, fmt.Errorf("%s must be a directory without group/other write permission, not a symlink", plane)
	}
	dir, err := base.OpenRoot(plane)
	if err != nil {
		return false, err
	}
	defer dir.Close()
	opened, err := dir.Stat(".")
	if err != nil {
		return false, err
	}
	if !os.SameFile(info, opened) {
		return false, fmt.Errorf("%s directory changed while reading PKI", plane)
	}
	entries, err := fs.ReadDir(dir.FS(), ".")
	if err != nil {
		return false, err
	}
	if len(entries) == 0 {
		return false, nil
	}
	files := make(map[string][]byte)
	for _, name := range []string{caCertFile, caKeyFile, clientCertFile, clientKeyFile} {
		files[name], err = readPlaneFile(dir, name)
		if err != nil {
			return false, fmt.Errorf("existing PKI is incomplete or unsafe; refusing to replace it: %w", err)
		}
	}
	if err := validatePlane(files); err != nil {
		return false, fmt.Errorf("existing PKI is invalid; refusing to replace it: %w", err)
	}
	return true, nil
}

func ensurePlane(base *os.Root, plane, caCN, clientCN string) error {
	if exists, err := existingPlane(base, plane); err != nil || exists {
		return err
	}
	pki, err := GeneratePlanePKI(caCN, clientCN)
	if err != nil {
		return err
	}
	// Publish all files together. Rename cannot replace a populated directory,
	// so concurrent installers cannot combine keys from different generations.
	tmp := "." + plane + "-" + rand.Text()
	if err := base.Mkdir(tmp, 0700); err != nil {
		return err
	}
	defer base.RemoveAll(tmp)
	for name, data := range map[string]*bytes.Buffer{
		caCertFile: pki.CACert, caKeyFile: pki.CAKey,
		clientCertFile: pki.ClientCrt, clientKeyFile: pki.ClientKey,
	} {
		f, err := base.OpenFile(filepath.Join(tmp, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, writeErr := f.Write(data.Bytes())
		closeErr := f.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			return err
		}
	}
	for attempt := 0; attempt < 2; attempt++ {
		err = base.Rename(tmp, plane)
		if err == nil {
			return nil
		}
		// Another installer may have completed this plane while we generated it.
		if exists, validationErr := existingPlane(base, plane); validationErr != nil {
			return validationErr
		} else if exists {
			return nil
		}
		// Some filesystems do not rename over an existing empty directory.
		// Remove only that empty destination; Remove fails if another installer
		// has populated it, and the next iteration validates that winner.
		if attempt == 0 {
			_ = base.Remove(plane)
		}
	}
	return err
}

// EnsureVMPlanePKI generates and persists both trust planes under baseDir.
// Complete, valid existing identities are preserved. Partial, expired, or
// mismatched PKI causes an error without replacing any existing material.
func EnsureVMPlanePKI(baseDir string) error {
	if baseDir == "" {
		baseDir = DefaultVMTLSBaseDir
	}
	if err := os.MkdirAll(baseDir, 0750); err != nil {
		return err
	}
	info, err := os.Lstat(baseDir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0022 != 0 {
		return errors.New("PKI base must be a directory without group/other write permission, not a symlink")
	}
	base, err := os.OpenRoot(baseDir)
	if err != nil {
		return err
	}
	defer base.Close()
	opened, err := base.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(info, opened) {
		return errors.New("PKI base directory changed while opening it")
	}
	if err := ensurePlane(base, VMPlaneManagement, VMManagementCACommonName, VMManagementClientCommonName); err != nil {
		return fmt.Errorf("cannot ensure management plane PKI: %w", err)
	}
	if err := ensurePlane(base, VMPlaneLog, VMLogCACommonName, VMLogClientCommonName); err != nil {
		return fmt.Errorf("cannot ensure log plane PKI: %w", err)
	}
	return nil
}
