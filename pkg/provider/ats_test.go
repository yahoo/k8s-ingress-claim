// Copyright 2017 Yahoo Holdings Inc.
// Licensed under the terms of the 3-Clause BSD License.
package provider

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
)

var (
	a = NewATSProvider()
)

func TestATSName(t *testing.T) {
	assert.Equal(t, a.Name(), ATS, "should return ATS")
}

func TestATSServesIngress(t *testing.T) {

	tests := []struct {
		name     string
		input    *v1beta1.Ingress
		expected bool
	}{
		{
			"should return false when annotation not present",
			&v1beta1.Ingress{},
			false,
		},
		{
			"should return false when annotation set to different provider",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						string(IngressClass): "other",
					},
				},
			},
			false,
		},
		{
			"should return true when ATS annotation is defined",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						string(IngressClass): ATS,
					},
				},
			},
			true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, a.ServesIngress(test.input), test.expected, test.name)
		})
	}
}

func TestATSGetDomains(t *testing.T) {

	tests := []struct {
		name     string
		input    *v1beta1.Ingress
		expected []string
	}{
		{
			"should return empty for an empty ingress spec",
			&v1beta1.Ingress{},
			[]string{},
		},
		{
			"should return the domains for an ingress with default domain and aliases",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-ingress",
					Annotations: map[string]string{
						string(DefaultDomain): "test1.company.com",
						string(Aliases):       "test2.company.com, test3.company.com",
						string(Ports):         "80",
					},
				},
			},
			[]string{
				"test1.company.com",
				"test2.company.com",
				"test3.company.com",
			},
		},
		{
			"should return the domains for an ingress only with default domain",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-ingress",
					Annotations: map[string]string{
						string(DefaultDomain): "test1.company.com",
					},
				},
			},
			[]string{
				"test1.company.com",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, a.GetDomains(test.input), test.name)
		})
	}
}

func TestATSDomainsIndexFunc(t *testing.T) {

	type output struct {
		domains []string
		err     error
	}
	tests := []struct {
		name     string
		input    interface{}
		expected output
	}{
		{
			"should return error for a non Ingress interface",
			&v1beta1.Deployment{
				Spec: v1beta1.DeploymentSpec{
					Paused: true,
				},
			},
			output{
				nil,
				errors.New("Resource is not an Ingress kind."),
			},
		},
		{
			"should return empty for an empty ingress spec",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:        "test-ingress",
					Annotations: map[string]string{},
				},
			},
			output{
				[]string{},
				nil,
			},
		},
		{
			"should return domains for an ATS ingress with domains",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-ingress",
					Annotations: map[string]string{
						string(DefaultDomain): "test1.company.com",
						string(Aliases):       "test2.company.com",
						string(Ports):         "80",
					},
				},
			},
			output{
				[]string{
					"test1.company.com",
					"test2.company.com",
				},
				nil,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var actual output
			actual.domains, actual.err = a.DomainsIndexFunc(test.input)
			assert.Equal(t, test.expected.err, actual.err, test.name)
			assert.Equal(t, test.expected.domains, actual.domains, test.name)
		})
	}
}

func TestATSValidateSemantics(t *testing.T) {

	tests := []struct {
		name     string
		input    *v1beta1.Ingress
		expected error
	}{
		{
			"should pass for a non ATS ingress spec",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-ingress",
					Annotations: map[string]string{
						string(IngressClass): Istio,
					},
				},
			},
			nil,
		},
		{
			"should pass for an ATS ingress with default domain, aliases and ports",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-ingress",
					Annotations: map[string]string{
						string(DefaultDomain): "test1.company.com",
						string(Aliases):       "test2.company.com",
						string(Ports):         "80",
					},
				},
				Spec: v1beta1.IngressSpec{
					Backend: &v1beta1.IngressBackend{
						ServiceName: "test2-svc",
						ServicePort: intstr.FromInt(80),
					},
				},
			},
			nil,
		},
		{
			"should fail for an ATS ingress without default backend",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress2",
					Namespace: "test-ns2",
					Annotations: map[string]string{
						string(DefaultDomain): "test1.company.com",
						string(Aliases):       "test2.company.com, test3.company.com",
						string(Ports):         "80",
					},
				},
			},
			errors.New("Ingress test-ingress2 in namespace test-ns2 does not have a default backend " +
				"specified."),
		},
		{
			"should fail for an ATS ingress without ports",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress2",
					Namespace: "test-ns2",
					Annotations: map[string]string{
						string(DefaultDomain): "test1.company.com",
						string(Aliases):       "test2.company.com, test3.company.com",
					},
				},
				Spec: v1beta1.IngressSpec{
					Backend: &v1beta1.IngressBackend{
						ServiceName: "test2-svc",
						ServicePort: intstr.FromInt(80),
					},
				},
			},
			errors.New("Ingress test-ingress2 in namespace test-ns2 does not have a ports annotation " +
				"specified."),
		},
		{
			"should fail for an ATS ingress without a default domain",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress2",
					Namespace: "test-ns2",
					Annotations: map[string]string{
						string(Aliases): "test2.company.com, test3.company.com",
						string(Ports):   "80",
					},
				},
				Spec: v1beta1.IngressSpec{
					Backend: &v1beta1.IngressBackend{
						ServiceName: "test2-svc",
						ServicePort: intstr.FromInt(80),
					},
				},
			},
			errors.New("Ingress test-ingress2 in namespace test-ns2 does not have a default_domain " +
				"annotation specified."),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := a.ValidateSemantics(test.input)
			if test.expected == nil {
				assert.Nil(t, err, test.name)
			} else if assert.NotNil(t, err, test.name) {
				assert.Equal(t, test.expected.Error(), err.Error(), test.name)
			}
		})
	}
}

func TestATSValidateDomainClaims(t *testing.T) {

	refIng := &v1beta1.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-ingress-ref",
			Namespace: "test-ns-ref",
			Annotations: map[string]string{
				string(DefaultDomain): "test-ref1.company.com",
				string(Aliases):       "test-ref2.company.com",
				string(Ports):         "80",
			},
		},
		Spec: v1beta1.IngressSpec{
			Backend: &v1beta1.IngressBackend{
				ServiceName: "test2-svc",
				ServicePort: intstr.FromInt(80),
			},
		},
	}
	refIstioIng := &v1beta1.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-istio-ingress-ref",
			Namespace: "test-ns-ref",
			Annotations: map[string]string{
				string(IngressClass): Istio,
			},
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: "test-istio-ref1.company.com",
				},
				{
					Host: "test-istio-ref2.company.com",
				},
			},
		},
	}
	helper.SetIndexer(cache.NewIndexer(
		cache.DeletionHandlingMetaNamespaceKeyFunc,
		cache.Indexers{
			ATS: helper.GetProviderByName(ATS).DomainsIndexFunc,
		}))
	helper.indexer.Add(refIng)
	helper.indexer.Add(refIstioIng)

	tests := []struct {
		name     string
		input    *v1beta1.Ingress
		expected error
	}{
		{
			"should pass for a non ATS ingress spec",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-ingress",
					Annotations: map[string]string{
						string(IngressClass): Istio,
					},
				},
			},
			nil,
		},
		{
			"should pass for an ATS ingress with no duplicate domains",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						string(DefaultDomain): "test1.company.com",
						string(Aliases):       "test2.company.com,test3.company.com",
						string(Ports):         "80",
					},
				},
				Spec: v1beta1.IngressSpec{
					Backend: &v1beta1.IngressBackend{
						ServiceName: "test2-svc",
						ServicePort: intstr.FromInt(80),
					},
				},
			},
			nil,
		},
		{
			"should pass for an ATS ingress update on same object",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress-ref",
					Namespace: "test-ns-ref",
					Annotations: map[string]string{
						string(DefaultDomain): "test-ref1.company.com",
						string(Aliases):       "test-ref2.company.com, test-ref3.company.com",
						string(Ports):         "80",
					},
				},
				Spec: v1beta1.IngressSpec{
					Backend: &v1beta1.IngressBackend{
						ServiceName: "test2-svc",
						ServicePort: intstr.FromInt(80),
					},
				},
			},
			nil,
		},
		{
			"should fail for an ATS ingress with duplicate domains",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						string(DefaultDomain): "test1.company.com",
						string(Aliases):       "test2.company.com,test-ref2.company.com",
						string(Ports):         "80",
					},
				},
				Spec: v1beta1.IngressSpec{
					Backend: &v1beta1.IngressBackend{
						ServiceName: "test2-svc",
						ServicePort: intstr.FromInt(80),
					},
				},
			},
			errors.New("Domain test-ref2.company.com already exists. Ingress test-ingress-ref in namespace " +
				"test-ns-ref owns this domain."),
		},
		{
			"should fail for an ATS ingress with duplicate domains on the same namespace",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "test-ns-ref",
					Annotations: map[string]string{
						string(DefaultDomain): "test1.company.com",
						string(Aliases):       "test2.company.com,test-ref2.company.com",
						string(Ports):         "80",
					},
				},
				Spec: v1beta1.IngressSpec{
					Backend: &v1beta1.IngressBackend{
						ServiceName: "test2-svc",
						ServicePort: intstr.FromInt(80),
					},
				},
			},
			errors.New("Domain test-ref2.company.com already exists. Ingress test-ingress-ref in namespace " +
				"test-ns-ref owns this domain."),
		},
		{
			"should pass for an ATS ingress with hosts same as Istio hosts",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress2",
					Namespace: "test-ns2",
					Annotations: map[string]string{
						string(DefaultDomain): "test-istio-ref1.company.com",
						string(Aliases):       "test-istio-ref2.company.com",
						string(Ports):         "80",
					},
				},
				Spec: v1beta1.IngressSpec{
					Backend: &v1beta1.IngressBackend{
						ServiceName: "test2-svc",
						ServicePort: intstr.FromInt(80),
					},
				},
			},
			nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := a.ValidateDomainClaims(test.input)
			if test.expected == nil {
				assert.Nil(t, err, test.name)
			} else if assert.NotNil(t, err, test.name) {
				assert.Equal(t, test.expected.Error(), err.Error(), test.name)
			}
		})
	}
	helper.indexer.Delete(refIng)
	helper.indexer.Delete(refIstioIng)
}
