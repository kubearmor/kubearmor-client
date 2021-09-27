# kArmor

**kArmor** is a CLI client to help manage [KubeArmor](github.com/kubearmor/KubeArmor)

KubeArmor is a container-aware runtime security enforcement system that
restricts the behavior (such as process execution, file access, and networking
operation) of containers at the system level.

## Installation

```
curl -sfL https://raw.githubusercontent.com/kubearmor/kubearmor-client/main/install.sh | sh
```

To build and install, clone the repository and
```
make install
```

## Usage

```
CLI Utility to help manage KubeArmor

Usage:
  karmor [command]

Available Commands:
  completion  generate the autocompletion script for the specified shell
  get         Display specified resources
  help        Help about any command
  install     Install KubeArmor in a Kubernetes Cluster
  log         Observe Logs from KubeArmor
  uninstall   Uninstall KubeArmor from a Kubernetes Cluster
  version     Display version information

Flags:
  -h, --help   help for karmor

Use "karmor [command] --help" for more information about a command.
```