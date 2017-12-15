# Copyright 2017 Yahoo Holdings Inc. Licensed under the terms of the 3-Clause BSD License.
FROM alpine:latest

COPY _build/bin/k8s-ingress-claim  /usr/bin/k8s-ingress-claim

CMD [ "k8s-ingress-claim", \
	"--admitAll=false", \
	"--keyFile=/etc/ssl/certs/k8s-ingress-claim/server-key.pem", \
	"--certFile=/etc/ssl/certs/k8s-ingress-claim/server.crt", \
	"--clientCAFile=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt", \
	"--clientAuth=false", \
	"--logFile=/var/log/k8s-ingress-claim.log", \
	"--logLevel=info", \
	"--port=443" ]