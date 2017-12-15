// Copyright 2017 Yahoo Holdings Inc.
// Licensed under the terms of the 3-Clause BSD License.
package provider

import (
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

type Annotation string

const (
	// IngressClass is the annotation on ingress resources for the class of controllers responsible for it
	IngressClass Annotation = "kubernetes.io/ingress.class"
)

type Provider interface {
	Name() string

	ServesIngress(ingress *v1beta1.Ingress) bool

	GetDomains(ingress *v1beta1.Ingress) []string

	DomainsIndexFunc(obj interface{}) ([]string, error)

	ValidateSemantics(ingress *v1beta1.Ingress) error

	ValidateDomainClaims(ingress *v1beta1.Ingress) error
}
