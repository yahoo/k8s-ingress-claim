// Copyright 2017 Yahoo Holdings Inc.
// Licensed under the terms of the 3-Clause BSD License.
package provider

import (
	"errors"
	"strings"

	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

const (
	ATS = "ATS"

	// ATS the default domain of an ingress resource
	DefaultDomain Annotation = "default_domain"

	// ATS domain aliases associated with the ingress resource
	Aliases Annotation = "aliases"

	// ATS Ports associated with the ingress resource
	Ports Annotation = "ports"
)

type ats struct{}

// NewATSProvider returns a new ATS provider ref that implements Provider interface
func NewATSProvider() Provider {
	return &ats{}
}

// Name returns "ATS"
func (ts *ats) Name() string {
	return ATS
}

// ServesIngress checks if the given ingress falls under ATS provider class
func (ts *ats) ServesIngress(ingress *v1beta1.Ingress) bool {
	class, exists := ingress.Annotations[string(IngressClass)]
	return !exists || class == ATS
}

// GetDomains returns the list of hosts associated with rules for the ATS ingress
func (ts *ats) GetDomains(ingress *v1beta1.Ingress) []string {
	domains := []string{}
	if ts.ServesIngress(ingress) {
		domains = helper.appendNonEmpty(domains, ts.getDefaultDomain(ingress))
		domains = helper.appendNonEmpty(domains, ts.getAliases(ingress)...)
	}
	return domains
}

// DomainsIndexFunc returns the list of hosts claimed by the given ATS ingress
func (ts *ats) DomainsIndexFunc(obj interface{}) ([]string, error) {
	domains := []string{}
	ingress, ok := obj.(*v1beta1.Ingress)
	if !ok {
		return nil, errors.New("Resource is not an Ingress kind.")
	}
	if ts.ServesIngress(ingress) {
		return ts.GetDomains(ingress), nil
	}
	return domains, nil
}

// ValidateSemantics performs ATS specific validation checks
func (ts *ats) ValidateSemantics(ingress *v1beta1.Ingress) error {
	if ts.ServesIngress(ingress) {
		if ingress.Spec.Backend == nil {
			return errors.New("Ingress " + ingress.Name + " in namespace " + ingress.Namespace +
				" does not have a default backend specified.")
		}

		if len(ts.getPorts(ingress)) == 0 {
			return errors.New("Ingress " + ingress.Name + " in namespace " + ingress.Namespace +
				" does not have a ports annotation specified.")
		}

		if ts.getDefaultDomain(ingress) == "" {
			return errors.New("Ingress " + ingress.Name + " in namespace " + ingress.Namespace +
				" does not have a default_domain annotation specified.")
		}
	}
	return nil
}

// ValidateDomainClaims checks if the ingress attempts to claim a "Domain" that has already been claimed
func (ts *ats) ValidateDomainClaims(ingress *v1beta1.Ingress) error {
	if ts.ServesIngress(ingress) {
		domains := ts.GetDomains(ingress)
		return helper.validateDomainClaims(ingress, domains)
	}
	return nil
}

// getDefaultDomain returns the sanitized domain specified for the "default_domain" annotation
func (ts *ats) getDefaultDomain(ingress *v1beta1.Ingress) string {
	annotationVal, exists := ingress.Annotations[string(DefaultDomain)]
	if exists {
		return helper.sanitize(annotationVal)
	}
	return ""
}

// getAliases returns the list of sanitized domains specified for the "aliases" annotation
func (ts *ats) getAliases(ingress *v1beta1.Ingress) []string {
	aliases := []string{}
	annotationVal, exists := ingress.Annotations[string(Aliases)]
	if !exists {
		return aliases
	}

	rawAliases := strings.Split(helper.sanitize(annotationVal), ",")
	return helper.appendNonEmpty(aliases, rawAliases...)
}

// getPorts returns the list of ports specified for the "ports" annotation
func (ts *ats) getPorts(ingress *v1beta1.Ingress) []string {
	ports := []string{}
	annotationVal, exists := ingress.Annotations[string(Ports)]
	if !exists {
		return ports
	}

	rawPorts := strings.Split(annotationVal, ",")
	return helper.appendNonEmpty(ports, rawPorts...)
}
