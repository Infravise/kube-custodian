# Kube Custodian

![Pipeline Status](https://gitlab.com/infravise/foundation/tooling/kube-custodian/badges/main/pipeline.svg)
![Test Coverage](https://gitlab.com/infravise/foundation/tooling/kube-custodian/badges/main/coverage.svg)
![Releases](https://gitlab.com/infravise/foundation/tooling/kube-custodian/-/badges/release.svg)

kube-custodian is an open-source Go application capable of cleaning epheneral resources on kubernetes through the use of labels in near real-time. In addition to this, kube-custodian is capable of automatically removing dangling/orphaned workloads that get left behind and tend to hog resources, preventing future workloads from being scheduled.

## Core Features

### TTL

When the `kube-custodian/ttl` label is added to a resource, kube-custodian will do the following:

1.  Fetch all resources containing the `kube-custodian/ttl` label
2.  Grab their values, add them to their resources respective creation timestamp, and assert them against the current time to determine whether or not they are stale
3.  Mark the stale resources for deletion
4.  Delete the stale resources from the cluster

The `kube-custodian/ttl` label currently supports the following options:

- `w` (weeks)
- `d` (days)
- `h` (hours)
- `m` (minutes)

Here are some examples:

In the following example, we want our resource(s) to be removed from the cluster 2 weeks after they've been deployed. This could be quite useful for those who are working on new application features, bug fixes, etc.. in an Agile sprint.

```
kube-custodian/ttl=2w
```

In the following example, we want our resource(s) to be removed from the cluster only 30 minuts after they've been deployed. This is particulary useful for small, consistent workloads that get spawned by another application like Gitlab runners.

```
kube-custodian/ttl=30m
```

In the following example, we want our resource(s) to be removed from the cluster 2 days, 12 hours, and 30 minutes after they've been deployed.

```
kube-custodian/ttl=2d12h30m
```

### Expiry Date

When the `kube-custodian/expires` label is added to a resource, kube-custodian will do the following:

1. Fetch all the resources containing the `kube-custodian/expires` label
2. Grab their values and assert them to the current time to determine whether or not they're expired
3. Mark the expired resources for deletion
4. Delete the expired resources from the cluster

The `kube-custodian/expires` label is expecting a [RFC3339](https://datatracker.ietf.org/doc/html/rfc3339) compliant timestamp to be provided.

Here are some examples:

In the following example, we want our resource(s) to be removed from the cluster on April 30th, 2025 at 12:00AM (UTC)

```
kube-custodian/expires=2025-04-30T00:00:00-00:00
```

In the following example, we want our resource(s) to be removed from the cluster on March 2nd, 2025 at 1:30PM (EST)

```
kube-custodian/expires=2025-03-02T13:30:00-05:00
```

### Dangling Pods

By default, kubernetes doesn't support the cleanup of `succeeded` or `failed` pods. Instead, we must do it ourselves manaully or resort to something else that automates this process. Normally, most folks create a bash script and run it as a CronJob on kubernetes that will cleanup the cluster every so often which works perfectly fine, but since this app is incredibly simple, lightweight, and a custodian after all, we decided to add it in anyway. When kube-custodian is deployed to the cluster, it will automatically do this without any additional configuration required. Here is what kube-custodian will do:

1. Fetch all the pods in the kubernetes cluster
2. Grab the pods with a `succeeded` or `failed` status/phase
3. Remove those pods from the cluster

## Installation & Deployment

The easiest method to deploy kube-custodian to your kubernetes cluster is to use our custom helm chart, to do this, following the steps listed below:

_NOTE: The chart version is always an exact match to the application version._

1. Download and update the helm repo:

```
helm repo add kube-custodian https://gitlab.com/api/v4/projects/65495019/packages/helm/api/
helm repo update
```

2. Customize your values file or use the chart's default.
3. Update your kubernetes context to target the right cluster.
4. Install the chart on the respective kubernetes cluster:

```
helm install kube-custodian kube-custodian/kube-custodian -f ./path/to/custom/values.yml -n kube-custodian --create-namespace
```
