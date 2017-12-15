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
	i = NewIstioProvider()

	testIngressRuleValue = v1beta1.IngressRuleValue{
		HTTP: &v1beta1.HTTPIngressRuleValue{
			Paths: []v1beta1.HTTPIngressPath{
				{
					Path: "/status",
					Backend: v1beta1.IngressBackend{
						ServiceName: "test2-svc",
						ServicePort: intstr.FromInt(80),
					},
				},
			},
		},
	}
)

func TestIstioName(t *testing.T) {
	assert.Equal(t, i.Name(), Istio, "should return istio")
}

func TestIstioServesIngress(t *testing.T) {

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
			"should return true when istio annotation is defined",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						string(IngressClass): Istio,
					},
				},
			},
			true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, i.ServesIngress(test.input), test.expected, test.name)
		})
	}
}

func TestIstioGetDomains(t *testing.T) {

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
			"should return the domains for an ingress with host rules",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-ingress",
					Annotations: map[string]string{
						string(IngressClass): Istio,
					},
				},
				Spec: v1beta1.IngressSpec{
					Rules: []v1beta1.IngressRule{
						{
							Host: "test1.company.com",
						},
						{
							Host: "test2.company.com",
						},
					},
				},
			},
			[]string{
				"test1.company.com",
				"test2.company.com",
			},
		},
		{
			"should return the domains for an ingress with host and non-host rules",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-ingress",
					Annotations: map[string]string{
						string(IngressClass): Istio,
					},
				},
				Spec: v1beta1.IngressSpec{
					Rules: []v1beta1.IngressRule{
						{
							Host: "test1.company.com",
						},
						{
							IngressRuleValue: testIngressRuleValue,
						},
						{
							Host: "test3.company.com",
						},
					},
				},
			},
			[]string{
				"test1.company.com",
				"test3.company.com",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, i.GetDomains(test.input), test.name)
		})
	}
}

func TestIstioDomainsIndexFunc(t *testing.T) {

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
					Name: "test-ingress",
					Annotations: map[string]string{
						string(IngressClass): Istio,
					},
				},
			},
			output{
				[]string{},
				nil,
			},
		},
		{
			"should return domains for an istio ingress with host rules",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-ingress",
					Annotations: map[string]string{
						string(IngressClass): Istio,
					},
				},
				Spec: v1beta1.IngressSpec{
					Rules: []v1beta1.IngressRule{
						{
							Host: "test1.company.com",
						},
						{
							Host: "test2.company.com",
						},
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
		{
			"should return domains for an istio ingress with host and non-host rules",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-ingress",
					Annotations: map[string]string{
						string(IngressClass): Istio,
					},
				},
				Spec: v1beta1.IngressSpec{
					Rules: []v1beta1.IngressRule{
						{
							Host: "test1.company.com",
						},
						{
							IngressRuleValue: testIngressRuleValue,
						},
						{
							Host: "test3.company.com",
						},
					},
				},
			},
			output{
				[]string{
					"test1.company.com",
					"test3.company.com",
				},
				nil,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var actual output
			actual.domains, actual.err = i.DomainsIndexFunc(test.input)
			assert.Equal(t, test.expected.err, actual.err, test.name)
			assert.Equal(t, test.expected.domains, actual.domains, test.name)
		})
	}
}

func TestIstioValidateSemantics(t *testing.T) {

	tests := []struct {
		name     string
		input    *v1beta1.Ingress
		expected error
	}{
		{
			"should pass for a non Istio ingress spec",
			&v1beta1.Ingress{},
			nil,
		},
		{
			"should pass for an istio ingress with host rules",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-ingress",
					Annotations: map[string]string{
						string(IngressClass): Istio,
					},
				},
				Spec: v1beta1.IngressSpec{
					Rules: []v1beta1.IngressRule{
						{
							Host: "test1.company.com",
						},
						{
							Host: "test2.company.com",
						},
					},
				},
			},
			nil,
		},
		{
			"should fail for an istio ingress with default backend",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress2",
					Namespace: "test-ns2",
					Annotations: map[string]string{
						string(IngressClass): Istio,
					},
				},
				Spec: v1beta1.IngressSpec{
					Backend: &v1beta1.IngressBackend{
						ServiceName: "test2-svc",
						ServicePort: intstr.FromInt(80),
					},
				},
			},
			errors.New("Ingress test-ingress2 in namespace test-ns2 specifies a default backend which is " +
				"currently NOT supported for provider class: " + Istio),
		},
		{
			"should fail for an istio ingress with host and non-host rules",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress2",
					Namespace: "test-ns2",
					Annotations: map[string]string{
						string(IngressClass): Istio,
					},
				},
				Spec: v1beta1.IngressSpec{
					Rules: []v1beta1.IngressRule{
						{
							Host: "test1.company.com",
						},
						{
							IngressRuleValue: testIngressRuleValue,
						},
						{
							Host: "test3.company.com",
						},
					},
				},
			},
			errors.New("Ingress test-ingress2 in namespace test-ns2 specifies an IngressRule without a " +
				"Host which is currently NOT supported for provider class: " + Istio),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := i.ValidateSemantics(test.input)
			if test.expected == nil {
				assert.Nil(t, err, test.name)
			} else if assert.NotNil(t, err, test.name) {
				assert.Equal(t, test.expected.Error(), err.Error(), test.name)
			}
		})
	}
}

func TestIstioValidateDomainClaims(t *testing.T) {

	refIng := &v1beta1.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-ingress-ref",
			Namespace: "test-ns-ref",
			Annotations: map[string]string{
				string(IngressClass): Istio,
			},
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: "test-ref1.company.com",
				},
				{
					Host: "test-ref2.company.com",
				},
			},
		},
	}
	refATSIng := &v1beta1.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-ats-ingress-ref",
			Namespace: "test-ns-ref",
			Annotations: map[string]string{
				string(DefaultDomain): "test-ats-ref1.company.com",
				string(Aliases):       "test-ats-ref2.company.com",
				string(Ports):         "80",
			},
		},
	}
	helper.SetIndexer(cache.NewIndexer(
		cache.DeletionHandlingMetaNamespaceKeyFunc,
		cache.Indexers{
			Istio: helper.GetProviderByName(Istio).DomainsIndexFunc,
		}))
	helper.indexer.Add(refIng)
	helper.indexer.Add(refATSIng)

	tests := []struct {
		name     string
		input    *v1beta1.Ingress
		expected error
	}{
		{
			"should pass for a non Istio ingress spec",
			&v1beta1.Ingress{},
			nil,
		},
		{
			"should pass for an istio ingress with no duplicate domains",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						string(IngressClass): Istio,
					},
				},
				Spec: v1beta1.IngressSpec{
					Rules: []v1beta1.IngressRule{
						{
							Host: "test1.company.com",
						},
						{
							Host: "test2.company.com",
						},
					},
				},
			},
			nil,
		},
		{
			"should pass for an istio ingress update on same object",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress-ref",
					Namespace: "test-ns-ref",
					Annotations: map[string]string{
						string(IngressClass): Istio,
					},
				},
				Spec: v1beta1.IngressSpec{
					Rules: []v1beta1.IngressRule{
						{
							Host: "test-ref1.company.com",
						},
						{
							Host: "test-ref2.company.com",
						},
						{
							Host: "test-ref3.company.com",
						},
					},
				},
			},
			nil,
		},
		{
			"should fail for an istio ingress with duplicate domains",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						string(IngressClass): Istio,
					},
				},
				Spec: v1beta1.IngressSpec{
					Rules: []v1beta1.IngressRule{
						{
							Host: "test1.company.com",
						},
						{
							Host: "test-ref2.company.com",
						},
					},
				},
			},
			errors.New("Domain test-ref2.company.com already exists. Ingress test-ingress-ref in namespace " +
				"test-ns-ref owns this domain."),
		},
		{
			"should fail for an istio ingress with duplicate domains on the same namespace",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "test-ns-ref",
					Annotations: map[string]string{
						string(IngressClass): Istio,
					},
				},
				Spec: v1beta1.IngressSpec{
					Rules: []v1beta1.IngressRule{
						{
							Host: "test1.company.com",
						},
						{
							Host: "test-ref2.company.com",
						},
					},
				},
			},
			errors.New("Domain test-ref2.company.com already exists. Ingress test-ingress-ref in namespace " +
				"test-ns-ref owns this domain."),
		},
		{
			"should pass for an istio ingress with hosts same as ATS domains",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress2",
					Namespace: "test-ns2",
					Annotations: map[string]string{
						string(IngressClass): Istio,
					},
				},
				Spec: v1beta1.IngressSpec{
					Rules: []v1beta1.IngressRule{
						{
							Host: "test-ats-ref1.company.com",
						},
						{
							Host: "test-ats-ref2.company.com",
						},
					},
				},
			},
			nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := i.ValidateDomainClaims(test.input)
			if test.expected == nil {
				assert.Nil(t, err, test.name)
			} else if assert.NotNil(t, err, test.name) {
				assert.Equal(t, test.expected.Error(), err.Error(), test.name)
			}
		})
	}
	helper.indexer.Delete(refIng)
	helper.indexer.Delete(refATSIng)
}
