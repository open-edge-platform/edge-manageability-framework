# EMF profiling: Analyze the timing and resource consumption of EMF with various profiles

Author(s): Krishna

Last updated: 05.06.2025

## Abstract

The objective of this exercise is to capture the timing and resource consumption of EMF when using different profiles. This will help in understanding the timing and resource consumption and guide future optimizations. For this exercise, we will use the EMF 3.0 release and coder instance.

## Tools used

- `kubectl top node` : Shows current real-time usage of CPU and memory.Data is fetched from the Metrics Server, which collects usage stats from kubelet.
- `kubectl describe node` : Shows the requested and allocatable resources, and also total capacity. This includes the sum of CPU and memory requests and limits from all pods on the node.
- Linux commands (instantaneous resource usage):
  - CPU: `top -bn1 | grep "Cpu(s)" | awk '{print "CPU Used: " $2 + $4 "%, CPU Free: " $8 "%"}'`
  - Memory: `free | awk '/Mem:/ {used=$3; total=$2; printf "Memory Used: %.2f%%, Memory Free: %.2f%%\n", used/total*100, $4/total*100}'`
  - Storage: `df --total -k | awk '/^total/ {used=$3; total=$2; free=$4; printf "Disk Used: %.2f%%, Disk Free: %.2f%%\n", used/total*100, free/total*100}'`
- instrumentation in the EMF code base

## Codebase and configuration

3.0 tag of EMF codebase is used for this exercise. EMF supports pre-defined deployment presets, which can be used to deploy the EMF instance with different profiles. The following profiles are used for this exercise: `dev-internal-coder-autocert.yaml` as baseline. Changes made to the presets will be captured as part of the profiling data.

## Profiling data

Two coder instance are deployed with same overall resource configuration

- CPU: 16 cores
- Memory: 64G
- Storage: 140G

### Comparing `dev-internal-coder-autocert.yaml` with `enableObservability` set to `false`

#### Kubernetes Resource allocation

Baseline profile `dev-internal-coder-autocert.yaml` with `enableObservability` set to `true`:

```sh
  Resource           Requests     Limits
  --------           --------     ------
  cpu                2466m (15%)  14957550m (93484%)
  memory             4603Mi (7%)  15308700Mi (24200%)  
```

```sh
NAME                 CPU(cores)   CPU%   MEMORY(bytes)   MEMORY%
kind-control-plane   4719m        29%    38049Mi         60%
```

Baseline profile `dev-internal-coder-autocert.yaml` with `enableObservability` set to `false`:

```sh
  Resource           Requests     Limits
  --------           --------     ------
  cpu                1823m (11%)  8646800m (54042%)
  memory             2169Mi (3%)  8850844Mi (13991%)
```

```sh
NAME                 CPU(cores)   CPU%   MEMORY(bytes)   MEMORY%
kind-control-plane   1507m        9%     19023Mi         30%
```

#### Linux Resource allocation

Baseline profile `dev-internal-coder-autocert.yaml` with `enableObservability` set to `true`:

```sh
CPU Used: 22.6%, CPU Free: 76.3%
Memory Used: 54.66%, Memory Free: 17.65%
Disk Used: 33.24%, Disk Free: 64.32%
```

Baseline profile `dev-internal-coder-autocert.yaml` with `enableObservability` set to `false`:

```sh
CPU Used: 6%, CPU Free: 94.0%
Memory Used: 24.48%, Memory Free: 62.97%
Disk Used: 22.89%, Disk Free: 74.31%
```
| Metric                | Observability ON         | Observability OFF        | % Delta Increase |
|-----------------------|-------------------------|--------------------------|------------------|
| **Kubernetes Requests** |                         |                          |                  |
| CPU (m)               | 2466                    | 1823                     | 35.3%            |
| Memory (Mi)           | 4603                    | 2169                     | 112.2%           |
| **Linux Usage**         |                         |                          |                  |
| CPU Used (%)          | 22.6                    | 6                        | 276.7%           |
| Memory Used (%)       | 54.66                   | 24.48                    | 123.4%           |
| Disk Used (%)         | 33.24                   | 22.89                    | 45.2%            |

Top processes consuming CPU and memory

Baseline profile `dev-internal-coder-autocert.yaml` with `enableObservability` set to `true`:

```sh
otelcol-contrib 47.7
kubelet         24.3
argocd-applicat 20.6
kube-apiserver  18.8
mimir            7.3
containerd       7.0 
```

```sh
COMMAND MEM(MiB)
kube-apiserver       1796.7
prometheus           1194.4
java                  797.8
mimir                 691.4
mimir                 690.0
argocd-applicat       658.2
```

Baseline profile `dev-internal-coder-autocert.yaml` with `enableObservability` set to `false`:

```sh
kubelet         15.9
kube-apiserver  13.8
argocd-applicat 12.9
etcd             5.3
containerd       4.6
envoy            1.6 
```

```sh
COMMAND MEM(MiB)
kube-apiserver       1625.4
java                  798.0
argocd-applicat       559.4
etcd                  249.7
containerd            235.5
kubelet               213.8
```


### Takeaways

- enabling observability in EMF significantly increases the resource consumption, especially in terms of CPU and memory.
