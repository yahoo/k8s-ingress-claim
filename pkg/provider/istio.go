// Copyright 2017 Yahoo Holdings Inc.
// Licensed under the terms of the 3-Clause BSD License.
package provider

import (
	"errors"

	"k8s.io/api/extensions/v1beta1"
)

const (
	Istio = "istio"
)

type istio struct{}

// NewIstioProvider returns a new istio provider ref that implements Provider interface
func NewIstioProvider() *istio {
	return &istio{}
}

// Name returns "istio"
func (i *istio) Name() string {
	return Istio
}

// ServesIngress checks if the given ingress falls under Istio provider class
func (i *istio) ServesIngress(ingress *v1beta1.Ingress) bool {
	class, exists := ingress.Annotations[string(IngressClass)]
	return exists && class == Istio
}

// GetDomains returns the list of hosts associated with rules for the Istio ingress
func (i *istio) GetDomains(ingress *v1beta1.Ingress) []string {
	hosts := []string{}
	if i.ServesIngress(ingress) {
		for _, rule := range ingress.Spec.Rules {
			hosts = helper.appendNonEmpty(hosts, rule.Host)
		}
	}
	return hosts
}

// DomainsIndexFunc returns the list of hosts claimed by the given Istio ingress
func (i *istio) DomainsIndexFunc(obj interface{}) ([]string, error) {
	ingress, ok := obj.(*v1beta1.Ingress)
	if !ok {
		return nil, errors.New("Resource is not an Ingress kind.")
	}
	if i.ServesIngress(ingress) {
		return i.GetDomains(ingress), nil
	}
	return []string{}, nil
}

// ValidateSemantics performs Istio specific validation checks
func (i *istio) ValidateSemantics(ingress *v1beta1.Ingress) error {
	if i.ServesIngress(ingress) {
		if ingress.Spec.Backend != nil {
			return errors.New("Ingress " + ingress.Name + " in namespace " + ingress.Namespace +
				" specifies a default backend which is currently NOT supported for provider class: " +
				Istio)
		}

		for _, rule := range ingress.Spec.Rules {
			if helper.sanitize(rule.Host) == "" {
				return errors.New("Ingress " + ingress.Name + " in namespace " + ingress.Namespace +
					" specifies an IngressRule without a Host which is currently NOT supported " +
					"for provider class: " + Istio)
			}
		}
	}
	return nil
}

// ValidateDomainClaims checks if the ingress attempts to claim a "Host" that has already been claimed
func (i *istio) ValidateDomainClaims(ingress *v1beta1.Ingress) error {
	if i.ServesIngress(ingress) {
		domains := i.GetDomains(ingress)
		return helper.validateDomainClaims(ingress, domains)
	}
	return nil
}
