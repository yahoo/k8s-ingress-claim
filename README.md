# k8s-ingress-claim

## Description
k8s-ingress-claim provides an admission control policy that safeguards against accidental duplicate claiming of 
Hosts/Domains by ingresses that have already been claimed by existing ingresses.

## Implementation
This is implemented as an [External Admission Webhook](https://kubernetes.io/docs/admin/extensible-admission-controllers/#external-admission-webhooks) 
with the k8s-ingress-claim service running as a deployment on each cluster.  

The webhook is configured to send admission review requests for *CREATE* and *UPDATE* operations on `ingress` resources
to the k8s-ingress-claim service. The k8s-ingress-claim service listens on a HTTPS port and on receiving such requests, 
it resolves the ingress claim provider for the new ingress resource and the provider implementation validates that no 
other existing ingresses own the hosts/domains being claimed. Every ingress claim provider may implement the validation 
to make sure the domain claims conform to its routing policies. 
   
This repository includes the domain claim validation check implementations for two ingress claim providers:
- Apache Traffic Server
- Istio

The example implementations on this repository assume that the ingresses claim domains on a FCFS basis.

The admission webhook service also provides a `ValidateSemantics` interface for the ingress claim provider to perform
provider specific semantic validation checks to ensure the ingress resources spec conform to policy specifications.

## Basic Dev Setup
1. Git clone to your local directory.
2. Build binary:
    - Mac os: `go build -i -o k8s-ingress-claim`
    - Rhel: `env GOOS=linux GOARCH=386 go build -i -o k8s-ingress-claim`
3. Run binary: `./k8s-ingress-claim`.
4. Follow standard Go code format: `gofmt -w *.go`

## Command Line Parameters
```
Usage of k8s-ingress-claim:
  -admitAll
    	True to admit all ingress without validation.
  -alsologtostderr
    	log to standard error as well as files
  -certFile string
    	The cert file for the https server. (default "/etc/ssl/certs/ingress-claim/server.crt")
  -clientAuth
    	True to verify client cert/auth during TLS handshake.
  -clientCAFile string
    	The cluster root CA that signs the apiserver cert (default "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
  -keyFile string
    	The key file for the https server. (default "/etc/ssl/certs/ingress-claim/server-key.pem")
  -logFile string
    	Log file name and full path. (default "/var/log/ingress-claim.log")
  -logLevel string
    	The log level. (default "info")
  -port string
    	HTTPS server port. (default "443")
```

Copyright 2017 Yahoo Holdings Inc. Licensed under the terms of the 3-Clause BSD License.
