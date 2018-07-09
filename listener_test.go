// Copyright 2017 Yahoo Holdings Inc.
// Licensed under the terms of the 3-Clause BSD License.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os/user"
	"testing"

	"github.com/yahoo/k8s-ingress-claim/pkg/provider"

	"github.com/stretchr/testify/assert"
	admv1beta1 "k8s.io/api/admission/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/cache"
)

var (
	templateIngress = &v1beta1.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-ingress",
			Namespace: "test-namespace",
			Annotations: map[string]string{
				string(provider.DefaultDomain): "app-domain-test.company.com",
				string(provider.Aliases): "app-domain-default.company.com, " +
					"app-domain-alias.company.com",
				string(provider.Ports): "80",
			},
		},
		Spec: v1beta1.IngressSpec{
			Backend: &v1beta1.IngressBackend{
				ServiceName: "test-svc",
				ServicePort: intstr.FromInt(80),
			},
		},
	}
	templateAdmReview = admv1beta1.AdmissionReview{
		Request: &admv1beta1.AdmissionRequest{
			Resource: v1.GroupVersionResource{
				Group:    "extensions",
				Version:  "v1beta1",
				Resource: "ingresses",
			},
			Kind: v1.GroupVersionKind{
				Kind: "Ingress",
			},
			Object: runtime.RawExtension{
				Raw: []byte("{}"),
			},
			Name:      "test-ingress",
			Namespace: "test-namespace",
			Operation: "CREATE",
			UserInfo: authenticationv1.UserInfo{
				Username: (func() string {
					user, err := user.Current()
					if err != nil {
						panic(err)
					}
					return user.Name
				})(),
			},
		},
		Response: &admv1beta1.AdmissionResponse{},
	}
)

func setIngressOnAdmissionReview(testAdmReview *admv1beta1.AdmissionReview, testIngress *v1beta1.Ingress) {
	ing := new(bytes.Buffer)
	err := json.NewEncoder(ing).Encode(testIngress)
	if err != nil {
		panic(err.Error())
	}
	testAdmReview.Request.Object.Raw = ing.Bytes()
}

func getAdmissionReview(rw *httptest.ResponseRecorder) *admv1beta1.AdmissionReview {
	admReview := &admv1beta1.AdmissionReview{
		Response: &admv1beta1.AdmissionResponse{},
		Request:  &admv1beta1.AdmissionRequest{},
	}
	err := json.NewDecoder(rw.Result().Body).Decode(admReview)
	if err != nil {
		panic(err.Error())
	}
	return admReview
}

func constructPostBody(admReview *admv1beta1.AdmissionReview) io.Reader {
	body := new(bytes.Buffer)
	err := json.NewEncoder(body).Encode(admReview)
	if err != nil {
		panic(err.Error())
	}
	return body
}

func TestAllowedWriteResponse(t *testing.T) {
	rw := httptest.NewRecorder()
	review := &admv1beta1.AdmissionReview{
		Request:  &admv1beta1.AdmissionRequest{},
		Response: &admv1beta1.AdmissionResponse{},
	}
	writeResponse(rw, review.Request, true, "")

	admReview := getAdmissionReview(rw)

	expectedAdmReview := &admv1beta1.AdmissionReview{
		Response: &admv1beta1.AdmissionResponse{
			Allowed: true,
			Result: &v1.Status{
				Reason: v1.StatusReason(""),
			},
		},
	}
	assert.Equal(t,
		expectedAdmReview.Response.Result.Status,
		admReview.Response.Result.Status,
		"writeResponse should write Allowed: true for AdmissionReviewStatus")
}

func TestNotAllowedWriteResponse(t *testing.T) {
	rw := httptest.NewRecorder()
	review := &admv1beta1.AdmissionReview{
		Request:  &admv1beta1.AdmissionRequest{},
		Response: &admv1beta1.AdmissionResponse{},
	}
	writeResponse(rw, review.Request, false, "Duplicate domain exists.")

	admReview := getAdmissionReview(rw)

	expectedAdmReview := &admv1beta1.AdmissionReview{
		Response: &admv1beta1.AdmissionResponse{
			Allowed: false,
			Result: &v1.Status{
				Reason: v1.StatusReason("Duplicate domain exists."),
			},
		},
	}
	assert.Equal(t,
		expectedAdmReview.Response.Result.Status,
		admReview.Response.Result.Status,
		"writeResponse should write Allowed: false for AdmissionReviewStatus")
}

func TestWrongMethodWebhookHandler(t *testing.T) {
	rw := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://localhost:8080/ingress", nil)

	webhookHandler(rw, req)

	assert.Equal(t, rw.Code, 405)
}

func TestWrongPathWebhookHandler(t *testing.T) {
	rw := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "http://localhost:8080/ingress", nil)

	webhookHandler(rw, req)

	assert.Equal(t, rw.Code, 404)
	body, err := ioutil.ReadAll(rw.Result().Body)
	assert.Nil(t, err, "Error should be nil")
	assert.Contains(t, string(body), "/ingress 404 Not Found")
}

func TestWrongReqBodyWebhookHandler(t *testing.T) {
	rw := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "http://localhost:8080/", nil)

	webhookHandler(rw, req)

	admReview := getAdmissionReview(rw)

	assert.False(t, admReview.Response.Allowed, "should fail if request doesn't have a body")
	assert.Contains(t, admReview.Response.Result.Reason, "Failed to decode the request body json into an "+
		"AdmissionReview resource: ")
}

func TestAdmitAllWebhookHandler(t *testing.T) {
	rw := httptest.NewRecorder()

	testSpec := templateAdmReview.DeepCopy()

	*admitAll = true

	req := httptest.NewRequest("POST", "http://localhost:8080/", constructPostBody(testSpec))
	webhookHandler(rw, req)

	admReview := getAdmissionReview(rw)

	assert.True(t, admReview.Response.Allowed, "should allow ingress to pass through if admitAll flag is set")
	*admitAll = false
}

func TestIngressResourceTypeWebhookHandler(t *testing.T) {
	rw := httptest.NewRecorder()

	testSpec := &admv1beta1.AdmissionReview{
		Request: &admv1beta1.AdmissionRequest{
			Resource: v1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
		},
	}

	req := httptest.NewRequest("POST", "http://localhost:8080/", constructPostBody(testSpec))
	webhookHandler(rw, req)

	admReview := getAdmissionReview(rw)

	assert.False(t, admReview.Response.Allowed, "should reject if the resource is not Ingress type")
	assert.Contains(t, admReview.Response.Result.Reason, "Incoming resource: { v1 pods} is not an Ingress resource")
}

func TestIngressDecodeWebhookHandler(t *testing.T) {
	rw := httptest.NewRecorder()

	testSpec := templateAdmReview.DeepCopy()
	testSpec.Request.Object.Raw = []byte("\"{}\"")

	body, _ := ioutil.ReadAll(constructPostBody(testSpec))
	bytes := new(bytes.Buffer)
	bytes.WriteString(string(body))

	req := httptest.NewRequest("POST", "http://localhost:8080/", bytes)
	webhookHandler(rw, req)

	admReview := getAdmissionReview(rw)

	assert.False(t, admReview.Response.Allowed, "should reject if the review object cannot be decoded to an Ingress")
	assert.Contains(t, admReview.Response.Result.Reason, "Failed to decode the raw object resource on the "+
		"admission review request into an Ingress resource: ")
}

func TestIngressValidationWebhookHandler(t *testing.T) {
	rw := httptest.NewRecorder()

	testSpec := templateAdmReview.DeepCopy()
	testIngress := templateIngress.DeepCopy()
	testIngress.Annotations[string(provider.Ports)] = ""
	setIngressOnAdmissionReview(testSpec, testIngress)

	req := httptest.NewRequest("POST", "http://localhost:8080/", constructPostBody(testSpec))
	webhookHandler(rw, req)

	admReview := getAdmissionReview(rw)

	assert.False(t, admReview.Response.Allowed, "should reject if the Ingress validation checks fail")
	assert.Contains(t, admReview.Response.Result.Reason, "Ingress validation checks failed: ")
}

func TestNoDuplicateDomainsWebhookHandler(t *testing.T) {
	rw := httptest.NewRecorder()

	testSpec := templateAdmReview.DeepCopy()
	testIngress := templateIngress.DeepCopy()
	testIngress2 := templateIngress.DeepCopy()
	testIngress2.Annotations[string(provider.DefaultDomain)] = "app-domain-default2.company.com"
	testIngress2.Annotations[string(provider.Ports)] = "443,80"
	testIngress2.Annotations[string(provider.Aliases)] = "app-domain-test2.company.com"
	testIngress2.Name = "second-ingress"
	testIngress2.Namespace = "second-namespace"

	indexer = cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc,
		cache.Indexers{provider.ATS: helper.GetProviderByName(provider.ATS).DomainsIndexFunc})
	indexer.Add(testIngress2)
	helper.SetIndexer(indexer)

	setIngressOnAdmissionReview(testSpec, testIngress)

	req := httptest.NewRequest("POST", "http://localhost:8080/", constructPostBody(testSpec))
	webhookHandler(rw, req)

	admReview := getAdmissionReview(rw)

	assert.True(t, admReview.Response.Allowed, "should approve if no duplicate domains found")
}

func TestNoDuplicateDomainsInSameIngressWebhookHandler(t *testing.T) {
	rw := httptest.NewRecorder()

	testSpec := templateAdmReview.DeepCopy()
	testIngress := templateIngress.DeepCopy()

	indexer = cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc,
		cache.Indexers{provider.ATS: helper.GetProviderByName(provider.ATS).DomainsIndexFunc})
	indexer.Add(testIngress)
	helper.SetIndexer(indexer)

	setIngressOnAdmissionReview(testSpec, testIngress)

	req := httptest.NewRequest("POST", "http://localhost:8080/", constructPostBody(testSpec))
	webhookHandler(rw, req)

	admReview := getAdmissionReview(rw)

	assert.True(t, admReview.Response.Allowed, "should approve even if domain exists within the same ingress object")
}

func TestDuplicateDomainsInSameNamespaceWebhookHandler(t *testing.T) {
	rw := httptest.NewRecorder()

	testSpec := templateAdmReview.DeepCopy()
	testIngress := templateIngress.DeepCopy()
	testIngress2 := templateIngress.DeepCopy()
	testIngress2.Annotations[string(provider.DefaultDomain)] = "app-domain-default.company.com"
	testIngress2.Annotations[string(provider.Ports)] = "443,80"
	testIngress2.Name = "second-ingress"
	testIngress2.Namespace = "test-namespace"

	indexer = cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc,
		cache.Indexers{provider.ATS: helper.GetProviderByName(provider.ATS).DomainsIndexFunc})
	indexer.Add(testIngress2)
	helper.SetIndexer(indexer)

	setIngressOnAdmissionReview(testSpec, testIngress)

	req := httptest.NewRequest("POST", "http://localhost:8080/", constructPostBody(testSpec))
	webhookHandler(rw, req)

	admReview := getAdmissionReview(rw)

	assert.False(t, admReview.Response.Allowed, "should reject if duplicate domain exists even within the same ns")
	assert.Contains(t, admReview.Response.Result.Reason, "Domain app-domain-default.company.com already "+
		"exists. Ingress second-ingress in namespace test-namespace owns this domain.")
}

func TestDuplicateDomainsWebhookHandler(t *testing.T) {
	rw := httptest.NewRecorder()

	testSpec := templateAdmReview.DeepCopy()
	testIngress := templateIngress.DeepCopy()
	testIngress2 := templateIngress.DeepCopy()
	testIngress2.Annotations[string(provider.DefaultDomain)] = "default-app-domain.company.com"
	testIngress2.Annotations[string(provider.Ports)] = "443,80"
	testIngress2.Annotations[string(provider.Aliases)] = "app-domain-alias.company.com"
	testIngress2.Name = "second-ingress"
	testIngress2.Namespace = "second-namespace"

	indexer = cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc,
		cache.Indexers{provider.ATS: helper.GetProviderByName(provider.ATS).DomainsIndexFunc})
	indexer.Add(testIngress2)
	helper.SetIndexer(indexer)

	setIngressOnAdmissionReview(testSpec, testIngress)

	req := httptest.NewRequest("POST", "http://localhost:8080/", constructPostBody(testSpec))
	webhookHandler(rw, req)

	admReview := getAdmissionReview(rw)

	assert.False(t, admReview.Response.Allowed, "should reject if duplicate domain exists on any other ns/ingress")
	assert.Contains(t, admReview.Response.Result.Reason, "Domain app-domain-alias.company.com already "+
		"exists. Ingress second-ingress in namespace second-namespace owns this domain.")
}

func TestStatusHandler200(t *testing.T) {
	rw := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://localhost:8080/status.html", nil)
	statusHandler(rw, req)
	assert.Equal(t, http.StatusOK, rw.Code, "/status.html should return 200")
}
