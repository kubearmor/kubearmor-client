[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/kubearmor/kubearmor-client/badge)](https://securityscorecards.dev/viewer/?uri=github.com/kubearmor/kubearmor-client)

# karmor

**karmor** is a client tool to help manage [KubeArmor](https://github.com/kubearmor/KubeArmor).

## Installation

```
curl -sfL http://get.kubearmor.io/ | sudo sh -s -- -b /usr/local/bin
```

### Installing from Source 

Build karmor from source if you want to test the latest (pre-release) karmor version.

```
git clone https://github.com/kubearmor/kubearmor-client.git
cd kubearmor-client
make install
```

## Usage

```
CLI Utility to help manage KubeArmor

KubeArmor is a container-aware runtime security enforcement system that
restricts the behavior (such as process execution, file access, and networking
operation) of containers at the system level.

Usage:
  karmor [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  discover    Discover applicable policies
  help        Help about any command
  install     Install KubeArmor in a Kubernetes Cluster
  logs        Observe Logs from KubeArmor
  probe       Checks for supported KubeArmor features in the current environment
  profile     Profiling of logs
  recommend   Recommend Policies
  rotate-tls  Rotate webhook controller tls certificates
  selfupdate  selfupdate this cli tool
  summary     Observability from discovery engine
  sysdump     Collect system dump information for troubleshooting and error report
  uninstall   Uninstall KubeArmor from a Kubernetes Cluster
  version     Display version information
  vm          VM commands for kvmservice

Flags:
      --context string      Name of the kubeconfig context to use
  -h, --help                help for karmor
      --kubeconfig string   Path to the kubeconfig file to use

Use "karmor [command] --help" for more information about a command.
```
