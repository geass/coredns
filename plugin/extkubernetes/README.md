# extkubernetes

## Name

*extkubernetes* - enables the reading external zone data from a Kubernetes cluster for Load Balancer IPs.

## Description

Creates A/AAAA, SRV, and PTR records for the External IPs of each LoadBalancer type Service in the Kubernetes cluster.


This plugin can only be used once per Server Block.

## Syntax

~~~
extkubernetes [ZONES...]
~~~

With only the directive specified, the *extkubernetes* plugin will default to the zones specified in
the server's block. It will handle all queries in those zones and connect to Kubernetes in-cluster.
If **ZONES** is used, it specifies all the zones the plugin should be authoritative for.

```
extkubernetes [ZONES...] {
    resyncperiod DURATION
    endpoint URL [URL...]
    tls CERT KEY CACERT
    kubeconfig KUBECONFIG CONTEXT
    namespaces NAMESPACE...
    labels EXPRESSION
    ttl TTL
    transfer to ADDRESS...
    fallthrough [ZONES...]
}
```

* `resyncperiod` specifies the Kubernetes data API **DURATION** period.
* `endpoint` specifies the **URL** for a remote k8s API endpoint.
   If omitted, it will connect to k8s in-cluster using the cluster service account.
   Multiple k8s API endpoints could be specified:
   `endpoint http://k8s-endpoint1:8080 http://k8s-endpoint2:8080`. CoreDNS
   will automatically perform a healthcheck and proxy to the healthy k8s API endpoint.
* `tls` **CERT** **KEY** **CACERT** are the TLS cert, key and the CA cert file names for remote k8s connection.
   This option is ignored if connecting in-cluster (i.e. endpoint is not specified).
* `kubeconfig` **KUBECONFIG** **CONTEXT** authenticates the connection to a remote k8s cluster using a kubeconfig file. It supports TLS, username and password, or token-based authentication. This option is ignored if connecting in-cluster (i.e. endpoint is not specified).
* `namespaces` **NAMESPACE [NAMESPACE...]**, only exposes the k8s namespaces listed.
   If this option is omitted all namespaces are exposed
* `labels` **EXPRESSION** only exposes the records for Kubernetes objects that match this label selector.
   The label selector syntax is described in the
   [Kubernetes User Guide - Labels](http://kubernetes.io/docs/user-guide/labels/). An example that
   only exposes objects labeled as "application=nginx" in the "staging" or "qa" environments, would
   use: `labels environment in (staging, qa),application=nginx`.
* `transfer` enables zone transfers. It may be specified multiples times. `To` signals the direction
  (only `to` is allow). **ADDRESS** must be denoted in CIDR notation (127.0.0.1/32 etc.) or just as
  plain addresses. The special wildcard `*` means: the entire internet.
  Sending DNS notifies is not supported.
  [Deprecated](https://github.com/kubernetes/dns/blob/master/docs/specification.md#26---deprecated-records) pod records in the sub domain `pod.cluster.local` are not transferred.
* `fallthrough` **[ZONES...]** If a query for a record in the zones for which the plugin is authoritative
  results in NXDOMAIN, normally that is what the response will be. However, if you specify this option,
  the query will instead be passed on down the plugin chain, which can include another plugin to handle
  the query. If **[ZONES...]** is omitted, then fallthrough happens for all zones for which the plugin
  is authoritative. If specific zones are listed (for example `in-addr.arpa` and `ip6.arpa`), then only
  queries for those zones will be subject to fallthrough.

## Health

This plugin implements dynamic health checking. Currently this is limited to reporting healthy when
the API has synced.

## Examples

Connect to Kubernetes with CoreDNS running outside the cluster:

~~~ txt
extkubernetes myzone.com {
    endpoint https://k8s-endpoint:8443
    tls cert key cacert
}
~~~

Handle all queries in the `myzone.com` zone. Connect to Kubernetes in-cluster. Also handle all
`in-addr.arpa` `PTR` requests for `123.0.0.0/17`. Note we show the entire server block here:

~~~ txt
123.0.0.0/17 myzone.com {
    extkubernetes
}
~~~

Or you can selectively expose namespaces.  In this case we expose the namespace "public":

~~~ txt
extkubernetes myzone.com {
    namespaces public
}
~~~

