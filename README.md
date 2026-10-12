[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/kubearmor/kubearmor-client/badge)](https://securityscorecards.dev/viewer/?uri=github.com/kubearmor/kubearmor-client)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fkubearmor%2Fkubearmor-client.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fkubearmor%2Fkubearmor-client?ref=badge_shield)

# karmor

**karmor** is a client tool to help manage [KubeArmor](https://github.com/kubearmor/KubeArmor).

## Installation

```shell
curl -sfL http://get.kubearmor.io/ | sh -s
```

### Installing From Source

Build karmor from source if you want to test the latest (pre-release) karmor version.

```shell
git clone https://github.com/kubearmor/kubearmor-client.git
cd kubearmor-client
make install
```

### Steps to Verify the Binary (Recommended)

We sign all releases with `cosign`, therefore we recommend verifying **karmor** tarball prior to its installation.

Below are the instructions to verify the binary using `cosign` for version `v1.1.0`.

- Use an environment variable to set the **karmor** version

```shell
export KARMOR_VERSION="1.1.0"
```

- Download released tarball, certificate, and signature files

<details>
  <summary>Download Details</summary>

```shell
curl -LO https://github.com/kubearmor/kubearmor-client/releases/download/v${KARMOR_VERSION}/karmor_${KARMOR_VERSION}_linux_amd64.tar.gz

curl -LO https://github.com/kubearmor/kubearmor-client/releases/download/v${KARMOR_VERSION}/karmor_${KARMOR_VERSION}_linux_amd64.tar.gz.cert

curl -LO https://github.com/kubearmor/kubearmor-client/releases/download/v${KARMOR_VERSION}/karmor_${KARMOR_VERSION}_linux_amd64.tar.gz.sig
```

</details>

- Verify the released tarball integrity with `cosign`

<details>
  <summary>Verification Details</summary>

```shell
cosign verify-blob karmor_${KARMOR_VERSION}_linux_amd64.tar.gz --certificate-identity=https://github.com/kubearmor/kubearmor-client/.github/workflows/release.yml@refs/tags/v${KARMOR_VERSION} --certificate-oidc-issuer=https://token.actions.githubusercontent.com --signature karmor_${KARMOR_VERSION}_linux_amd64.tar.gz.sig --certificate karmor_${KARMOR_VERSION}_linux_amd64.tar.gz.cert
```

</details>

## Usage

```shell
CLI Utility to help manage KubeArmor

KubeArmor is a container-aware runtime security enforcement system that
restricts the behavior (such as process execution, file access, and networking
operation) of containers at the system level.

Usage:
  karmor [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  install     Install KubeArmor in a Kubernetes Cluster
  logs        Observe Logs from KubeArmor
  probe       Checks for supported KubeArmor features in the current environment
  profile     Profiling of logs
  recommend   Recommend Policies
  rotate-tls  Rotate webhook controller tls certificates
  selfupdate  selfupdate this cli tool
  sysdump     Collect system dump information for troubleshooting and error report
  uninstall   Uninstall KubeArmor from a Kubernetes Cluster
  version     Display version information
  vm          VM commands for non kubernetes/bare metal KubeArmor

Flags:
      --context string      Name of the kubeconfig context to use
  -h, --help                help for karmor
      --kubeconfig string   Path to the kubeconfig file to use

Use "karmor [command] --help" for more information about a command.
```

### Standalone agents

`karmor install --nonk8s --vm-mode docker` (or `--vm-mode systemd`) provisions
independent management and log certificate authorities under
`/var/lib/kubearmor/tls/management` and `/var/lib/kubearmor/tls/log`. Each directory
contains `ca.crt`, `ca.key`, `client.crt`, and `client.key`. Installation requires
permission to create these files. Existing valid identities are reused; incomplete
or invalid material produces an error without silently rotating the CA.

The agent uses its shared `tlsCertProvider=self` configuration to issue server
certificates, with `managementTLSCertPath` and `tlsCertPath` selecting the two
independent directories. The CLI uses the persisted client certificates; it only
needs `ca.crt`, `client.crt`, and `client.key` for the endpoint being accessed.
Keep CA private keys on the agent host when copying credentials to another client.

```shell
# Management endpoint: localhost:32765
karmor vm policy add ./policy.yaml
karmor vm policy delete ./policy.yaml
karmor probe --nonk8s

# Log endpoint: localhost:32767
karmor logs --nonk8s --json
```

These commands work without a kubeconfig. `--nonk8s` on `logs` always uses mutual
TLS and locally stored client credentials. For a different log credential
directory, use `--tlsCertPath /path/to/log`; the parent directory containing `log/`
is also accepted. Kubernetes logging keeps its existing defaults.

Policy and standalone probe commands accept `--gRPC host:port` and
`--managementTLSCertPath /path/to/management`. Their environment variable
equivalents are `KUBEARMOR_MANAGEMENT_SERVICE` and
`KUBEARMOR_MANAGEMENT_TLS_CERT_PATH`. Logs accept `--gRPC host:port` or
`KUBEARMOR_SERVICE`. The server certificate must cover the requested hostname or
IP address. TLS failures are returned to the caller; policy requests never retry
against the plaintext log port.

The Docker installation publishes management port 32765 on `127.0.0.1` only.
Remote administration requires an explicitly configured listener or tunnel and
the management client identity. These management commands require an agent with
the dedicated management server; older agents are not selected automatically.

## License

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fkubearmor%2Fkubearmor-client.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fkubearmor%2Fkubearmor-client?ref=badge_large)
