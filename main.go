// Copyright 2017 Yahoo Holdings Inc.
// Licensed under the terms of the 3-Clause BSD License.
package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/yahoo/k8s-ingress-claim/pkg/provider"
	"github.com/yahoo/k8s-ingress-claim/pkg/util"

	"github.com/Sirupsen/logrus"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

var (
	port          = flag.String("port", "443", "HTTPS server port.")
	logFilename   = flag.String("logFile", "/var/log/k8s-ingress-claim.log", "Log file name and full path.")
	logLevel      = flag.String("logLevel", "info", "The log level.")
	httpsCertFile = flag.String("certFile", "/etc/ssl/certs/k8s-ingress-claim/server.crt", "The cert file for the https server.")
	httpsKeyFile  = flag.String("keyFile", "/etc/ssl/certs/k8s-ingress-claim/server-key.pem", "The key file for the https server.")
	clientCAFile  = flag.String("clientCAFile", "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt", "The cluster root CA that signs the apiserver cert")
	clientAuth    = flag.Bool("clientAuth", false, "True to verify client cert/auth during TLS handshake.")
	admitAll      = flag.Bool("admitAll", false, "True to admit all ingress without validation.")

	indexer  cache.Indexer
	informer cache.Controller

	helper = provider.GetHelper()

	log *logrus.Logger
)

func init() {
	flag.Parse()
	log = util.GetLogger(*logFilename, *logLevel)
}

func main() {

	// creates the k8s in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	// create the ingress watcher
	ingressListWatcher := cache.NewListWatchFromClient(clientset.ExtensionsV1beta1().RESTClient(),
		"ingresses",
		v1.NamespaceAll,
		fields.Everything())

	// create the indexer & informer framework
	indexer, informer = cache.NewIndexerInformer(ingressListWatcher,
		&v1beta1.Ingress{},
		0,
		cache.ResourceEventHandlerFuncs{},
		cache.Indexers{
			provider.ATS:   helper.GetProviderByName(provider.ATS).DomainsIndexFunc,
			provider.Istio: helper.GetProviderByName(provider.Istio).DomainsIndexFunc,
		})

	helper.SetIndexer(indexer)

	// start the informer before calling handlers (dependency: indexer)
	stop := make(chan struct{})
	log.Info("Starting Ingress informer...")
	go informer.Run(stop)

	// wait for all involved cache to be synced, before processing items from the queue is started
	log.Debugf("Waiting for the cache to be synced...")
	if !cache.WaitForCacheSync(stop, informer.HasSynced) {
		log.Fatal(fmt.Errorf("Timed out waiting for the cache to sync"))
	}

	// add the serving path handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/status.html", statusHandler)
	mux.HandleFunc("/", webhookHandler)

	// load the https server cert and key
	xcert, err := tls.LoadX509KeyPair(*httpsCertFile, *httpsKeyFile)
	if err != nil {
		log.Fatalf("Unable to read the server cert and/or key file: %s", err.Error())
	}

	// load the cluster CA that signs the client(apiserver) cert
	caCert, err := ioutil.ReadFile(*clientCAFile)
	if err != nil {
		log.Fatalf("Unable to read the client CA cert file: %s", err.Error())
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// create the TLS config for the https server
	tlsConfig := &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{xcert},
		ClientCAs:    caCertPool,
	}

	// enable client(apiserver) certificate verification if --clientAuth=true
	if *clientAuth {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	// create the https server object
	srv := &http.Server{
		Addr:      ":" + *port,
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	// start the https server
	go func() {
		err = srv.ListenAndServeTLS("", "")
		if err != nil {
			log.Fatal(err)
		}
	}()
	log.Infof("HTTPS server listening on port:%s with ClientAuthEnabled:%t ", *port, *clientAuth)

	// graceful shutdown..
	signalChan := make(chan os.Signal, 2)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-signalChan:
			log.Printf("Shutdown signal received, exiting...")
			close(stop)
			os.Exit(0)
		}
	}
}
