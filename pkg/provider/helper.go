// Copyright 2017 Yahoo Holdings Inc.
// Licensed under the terms of the 3-Clause BSD License.
package provider

import (
	"fmt"
	"strings"

	"k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
)

var (
	helper *Helper
)

// Helper class that provides common validation funcs and a handle to
// ingress claim provider implementations
type Helper struct {
	providers map[string]Provider
	indexer   cache.Indexer
}

// init sets-up the provider instances
func init() {
	helper = &Helper{
		providers: map[string]Provider{
			ATS:   NewATSProvider(),
			Istio: NewIstioProvider(),
		},
	}
}

// GetHelper returns the singleton helper instance
func GetHelper() *Helper {
	return helper
}

// GetDefaultProvider returns the default ingress claim provider instance
func (h *Helper) GetDefaultProvider() Provider {
	return h.providers[ATS]
}

// GetProvider returns the provider instance corresponding to the given ingress resource
func (h *Helper) GetProvider(ingress *v1beta1.Ingress) Provider {
	for _, provider := range h.providers {
		if provider.ServesIngress(ingress) {
			return provider
		}
	}
	return h.GetDefaultProvider()
}

// GetProviderByName returns a handle to the provider instance by the provider name
func (h *Helper) GetProviderByName(name string) Provider {
	return h.providers[name]
}

// SetIndexer allows to set the cache indexer to be used for lookups by helper funcs
// This is not done in `init` to allow lazy set once the cache indexer is configured
func (h *Helper) SetIndexer(indexer cache.Indexer) {
	h.indexer = indexer
}

// sanitize strips the whitespaces in a string
func (h *Helper) sanitize(s string) string {
	return strings.Replace(strings.ToLower(s), " ", "", -1)
}

// appendNonEmpty appends an item to a string slice only if it's non-empty
func (h *Helper) appendNonEmpty(slice []string, items ...string) []string {
	for _, item := range items {
		item = h.sanitize(item)
		if item != "" {
			slice = append(slice, item)
		}
	}
	return slice
}

// lookupIngressesByDomain provides a lookup on the cache index with the name 'index'
// on the 'domain', this assumes SetIndexer has been called previously
func (h *Helper) lookupIngressesByDomain(index string, domain string) (ingresses [](*v1beta1.Ingress), err error) {
	matches, err := h.indexer.ByIndex(index, domain)
	if err != nil {
		return ingresses, err
	}
	for _, match := range matches {
		if ingress, ok := match.(*v1beta1.Ingress); ok {
			ingresses = append(ingresses, ingress)
		}
	}
	return ingresses, nil
}

// validateDomainClaims provides a helper function to perform the duplicate domain check
// in a provider agnostic manner
func (h *Helper) validateDomainClaims(ingress *v1beta1.Ingress, domains []string) error {
	for _, domain := range domains {
		ingressMatches, err := h.lookupIngressesByDomain(h.GetProvider(ingress).Name(), domain)
		if err != nil {
			return err
		}

		for _, ingressMatch := range ingressMatches {
			if !(ingressMatch.Namespace == ingress.Namespace && ingressMatch.Name == ingress.Name) {
				return fmt.Errorf("Domain %s already exists. Ingress %s in namespace %s owns "+
					"this domain.", domain, ingressMatch.Name, ingressMatch.Namespace)
			}
		}
	}
	return nil
}
