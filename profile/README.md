## Profiling of Kubearmor Logs using karmor

`karmor profile` which shows real-time terminal user interface table of three different operations going on in KubeArmor: Process, File and Network. It maintains a counter of each operation that is happening within the cluster, along with other useful details. It directly fetches data from the `karmor logs` API and displays all the required information. The TUI includes simple navigation between operations and a user input based filter as well.

![Profile](https://user-images.githubusercontent.com/23097199/213850468-2462e8b2-b4f6-491f-a174-42d217cbfd28.gif)


### 🔍 Filtering Logs with `karmor profile`

The `karmor profile` command allows you to filter logs or alerts using a set of useful flags. These filters help narrow down the output to specific Kubernetes objects like containers, pods, and namespaces.

### 🧰 Available Filters

| Flag                | Description                               |
| ------------------- | ----------------------------------------- |
| `-c`, `--container` | Filters logs by **container name**.       |
| `-n`, `--namespace` | Filters logs by **Kubernetes namespace**. |
| `--pod`             | Filters logs by **pod name**.             |

---

### 📌 Usage Examples

#### ✅ Filter by Container Name

```bash
karmor profile -c nginx
```

> Outputs logs only from the container named `nginx`.

---

#### ✅ Filter by Namespace

```bash
karmor profile -n nginx1
```

> Outputs logs only from the namespace `nginx1`.

---

#### ✅ Filter by Pod

```bash
karmor profile --pod nginx-pod-1
```

> Outputs logs only from the pod named `nginx-pod-1`.

---

### 🔗 Combine Multiple Filters

You can combine filters to narrow down the logs even further.

```bash
karmor profile -n nginx1 -c nginx
```

> Outputs logs **only** from the `nginx` container in the `nginx1` namespace.

---

### 💡 Tip

Use these filters during profiling sessions to quickly isolate behavior or security events related to a specific pod, container, or namespace.
