// Copyright 2017 Yahoo Holdings Inc.
// Licensed under the terms of the 3-Clause BSD License.
package provider

import (
	"fmt"
	"testing"

	"errors"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
)

func TestGetDefaultProvider(t *testing.T) {
	if assert.NotNil(t, helper.GetDefaultProvider(), "should not be nil") {
		assert.Equal(t, helper.GetDefaultProvider().Name(), ATS, "should return ATS")
	}
}

func TestGetProvider(t *testing.T) {
	tests := []struct {
		name     string
		input    *v1beta1.Ingress
		expected string
	}{
		{
			"should return default(ATS) provider for empty Ingress",
			&v1beta1.Ingress{},
			ATS,
		},
		{
			"should return ATS provider when annotation set to different provider",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						string(IngressClass): "other",
					},
				},
			},
			ATS,
		},
		{
			"should return Istio provider when istio annotation is defined",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						string(IngressClass): Istio,
					},
				},
			},
			Istio,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := helper.GetProvider(test.input)
			if assert.NotNil(t, p, "provider is nil: "+test.name) {
				assert.Equal(t, p.Name(), test.expected, test.name)
			}
		})
	}
}

func TestGetProviderByName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"should return ATS provider",
			ATS,
			ATS,
		},
		{
			"should return Istio provider",
			Istio,
			Istio,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := helper.GetProviderByName(test.input)
			if assert.NotNil(t, p, "provider is nil: "+test.name) {
				assert.Equal(t, p.Name(), test.expected, test.name)
			}
		})
	}
}

func TestSetIndexer(t *testing.T) {
	i := cache.NewIndexer(
		cache.DeletionHandlingMetaNamespaceKeyFunc,
		cache.Indexers{})
	helper.SetIndexer(i)
	assert.Equal(t, i, helper.indexer, "should set the indexer")
}

func TestSanitize(t *testing.T) {
	assert.Equal(t, helper.sanitize("   "), "")
	assert.Equal(t, helper.sanitize("a.y.c, b.y.c"), "a.y.c,b.y.c")
	assert.Equal(t, helper.sanitize(" 80, 4080 "), "80,4080")
	assert.Equal(t, helper.sanitize("aA.y.c, Bb.y.c"), "aa.y.c,bb.y.c")
}

func TestAppendNonEmpty(t *testing.T) {
	tests := []struct {
		input    []string
		expected []string
	}{
		{
			[]string{""},
			[]string{},
		},
		{
			[]string{" "},
			[]string{},
		},
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("case #%d", i), func(t *testing.T) {
			assert.Equal(t, test.expected, helper.appendNonEmpty([]string{}, test.input...))
		})
	}
}

func TestLookupIngressesByDomain(t *testing.T) {
	refIng1 := &v1beta1.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-ingress-ref1",
			Namespace: "test-ns-ref",
			Annotations: map[string]string{
				string(IngressClass): Istio,
			},
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: "test-ref1.abc.company.com",
				},
				{
					Host: "test-ref1.xyz.company.com",
				},
			},
		},
	}
	refIng2 := &v1beta1.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-ingress-ref2",
			Namespace: "test-ns-ref",
			Annotations: map[string]string{
				string(IngressClass): Istio,
			},
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: "test-ref2.abc.company.com",
				},
			},
		},
	}
	refIng3 := &v1beta1.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-ingress-ref3",
			Namespace: "test-ns-ref3",
			Annotations: map[string]string{
				string(IngressClass): Istio,
			},
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: "test-ref1.abc.company.com",
				},
				{
					Host: "test-ref3.xyz.company.com",
				},
			},
		},
	}
	helper.SetIndexer(cache.NewIndexer(
		cache.DeletionHandlingMetaNamespaceKeyFunc,
		cache.Indexers{
			Istio: helper.GetProviderByName(Istio).DomainsIndexFunc,
		}))
	helper.indexer.Add(refIng1)
	helper.indexer.Add(refIng2)
	helper.indexer.Add(refIng3)

	type input struct {
		index  string
		domain string
	}
	type output struct {
		ingresses [](*v1beta1.Ingress)
		err       error
	}
	tests := []struct {
		name     string
		given    input
		expected output
	}{
		{
			"should return error when index doesn't exist",
			input{
				"undefined",
				"test-ref1.abc.company.com",
			},
			output{
				nil,
				errors.New("undefined"),
			},
		},
		{
			"should return correct ingress for test-ref1.xyz.company.com",
			input{
				Istio,
				"test-ref1.xyz.company.com",
			},
			output{
				[](*v1beta1.Ingress){
					refIng1,
				},
				nil,
			},
		},
		{
			"should return correct ingress for test-ref2.abc.company.com",
			input{
				Istio,
				"test-ref2.abc.company.com",
			},
			output{
				[](*v1beta1.Ingress){
					refIng2,
				},
				nil,
			},
		},
		{
			"should return correct multiple ingresses for test-ref1.abc.company.com",
			input{
				Istio,
				"test-ref1.abc.company.com",
			},
			output{
				[](*v1beta1.Ingress){
					refIng1,
					refIng3,
				},
				nil,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var actual output
			actual.ingresses, actual.err = helper.lookupIngressesByDomain(test.given.index,
				test.given.domain)
			if test.expected.err != nil {
				assert.NotNil(t, actual.err, "err should not be nil: "+test.name)
			} else {
				assert.Nil(t, actual.err, "err should be nil: "+test.name)
			}
			assert.Equal(t, test.expected.ingresses, actual.ingresses, test.name)
		})
	}
}

func TestValidateDomainClaims(t *testing.T) {

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
			ATS:   helper.GetProviderByName(ATS).DomainsIndexFunc,
			Istio: helper.GetProviderByName(Istio).DomainsIndexFunc,
		}))
	helper.indexer.Add(refATSIng)
	helper.indexer.Add(refIstioIng)

	tests := []struct {
		name     string
		input    *v1beta1.Ingress
		expected error
	}{
		{
			"should pass for an empty ingress spec",
			&v1beta1.Ingress{},
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
			"should pass for an ATS ingress update on same object",
			&v1beta1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-ats-ingress-ref",
					Namespace: "test-ns-ref",
					Annotations: map[string]string{
						string(DefaultDomain): "test-ats-ref1.company.com",
						string(Aliases):       "test-ats-ref2.company.com, test-ats-ref3.company.com",
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
			"should pass for an istio ingress update on same object",
			&v1beta1.Ingress{
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
						{
							Host: "test-istio-ref3.company.com",
						},
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
						string(Aliases):       "test2.company.com,test-ats-ref2.company.com",
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
			errors.New("Domain test-ats-ref2.company.com already exists. Ingress test-ats-ingress-ref in " +
				"namespace test-ns-ref owns this domain."),
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
							Host: "test-istio-ref2.company.com",
						},
					},
				},
			},
			errors.New("Domain test-istio-ref2.company.com already exists. Ingress test-istio-ingress-ref " +
				"in namespace test-ns-ref owns this domain."),
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
			err := helper.validateDomainClaims(test.input,
				helper.GetProvider(test.input).GetDomains(test.input))
			if test.expected == nil {
				assert.Nil(t, err, test.name)
			} else if assert.NotNil(t, err, test.name) {
				assert.Equal(t, test.expected.Error(), err.Error(), test.name)
			}
		})
	}
	helper.indexer.Delete(refIstioIng)
	helper.indexer.Delete(refATSIng)
}
