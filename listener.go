// Copyright 2017 Yahoo Holdings Inc.
// Licensed under the terms of the 3-Clause BSD License.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"k8s.io/api/admission/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

var (
	ingressResourceType = v1.GroupVersionResource{
		Group:    "extensions",
		Version:  "v1beta1",
		Resource: "ingresses",
	}
)

// writeResponse writes the ingressReviewStatus object to the response body
func writeResponse(rw http.ResponseWriter, admReview *v1alpha1.AdmissionReview, allowed bool, errorMsg string) {
	log.Infof("Responding Allowed: %t for %s on Ingress: %s/%s by user: %s", allowed,
		admReview.Spec.Operation,
		admReview.Spec.Namespace,
		admReview.Spec.Name,
		admReview.Spec.UserInfo.Username)

	if !allowed {
		log.Errorf("Rejection reason: %s", errorMsg)
	}

	admReview.Status = v1alpha1.AdmissionReviewStatus{
		Allowed: allowed,
		Result: &v1.Status{
			Reason: v1.StatusReason(errorMsg),
		},
	}

	body := new(bytes.Buffer)
	err := json.NewEncoder(body).Encode(admReview)
	if err != nil {
		io.WriteString(rw, "Error occurred while encoding the admission review status into json: "+err.Error())
		return
	}
	rw.Write(body.Bytes())
}

// webhookHandler serves all the CREATE and UPDATE admission webhook calls on ingress resources and returns the
// AdmissionReviewSpec with the admission status determined based on the validation and domain claims check results
func webhookHandler(rw http.ResponseWriter, req *http.Request) {
	log.Infof("Serving %s %s request for client: %s", req.Method, req.URL.Path, req.RemoteAddr)

	if req.Method != http.MethodPost {
		http.Error(rw, fmt.Sprintf("Incoming request method %s is not supported, only POST is supported",
			req.Method), http.StatusMethodNotAllowed)
		return
	}

	if req.URL.Path != "/" {
		http.Error(rw, fmt.Sprintf("%s 404 Not Found", req.URL.Path), http.StatusNotFound)
		return
	}

	admReview := &v1alpha1.AdmissionReview{}
	err := json.NewDecoder(req.Body).Decode(admReview)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to decode the request body json into an AdmissionReview resource: %s",
			err.Error())
		writeResponse(rw, admReview, false, errorMsg)
		return
	}
	log.Debugf("Incoming AdmissionReview for resource: %v, kind: %v", admReview.Spec.Resource, admReview.Spec.Kind)

	// when bypass flag is set, all the admission webhook calls return true unconditionally
	if *admitAll == true {
		log.Warnf("admitAll flag is set to true. Allowing Ingress admission review request to pass through " +
			"without validation.")
		writeResponse(rw, admReview, true, "")
		return
	}

	if admReview.Spec.Resource != ingressResourceType {
		errorMsg := fmt.Sprintf("Incoming resource: %v is not an Ingress resource", admReview.Spec.Resource)
		writeResponse(rw, admReview, false, errorMsg)
		return
	}

	// decode the incoming object into an ingress resource
	ingress := &v1beta1.Ingress{}
	if err := json.Unmarshal(admReview.Spec.Object.Raw, ingress); err != nil {
		errorMsg := fmt.Sprintf("Failed to decode the raw object resource on the admission review request "+
			"into an Ingress resource: %s", err.Error())
		writeResponse(rw, admReview, false, errorMsg)
		return
	}
	log.Debugf("Decoded Ingress spec %v", ingress)

	if err := json.Unmarshal(admReview.Spec.Object.Raw, &ingress.ObjectMeta); err != nil {
		errorMsg := fmt.Sprintf("Failed to parse the Ingress metadata from the raw object resource on the "+
			"admission review request: %s", err.Error())
		writeResponse(rw, admReview, false, errorMsg)
		return
	}
	log.Debugf("Decoded Ingress metadata %v", ingress.ObjectMeta)

	// retrieve the ingress claim provider implementation for the current resource
	p := helper.GetProvider(ingress)

	// perform the ingress claim provider specific validation checks
	err = p.ValidateSemantics(ingress)
	if err != nil {
		errorMsg := fmt.Sprintf("Ingress validation checks failed: %s", err.Error())
		writeResponse(rw, admReview, false, errorMsg)
		return
	}

	// perform the domain claims check with the ingress provider
	err = p.ValidateDomainClaims(ingress)
	if err != nil {
		writeResponse(rw, admReview, false, err.Error())
		return
	}

	log.Infof("Ingress %s in namespace %s contains no duplicate domains.", ingress.Name, ingress.Namespace)
	writeResponse(rw, admReview, true, "")
}

// statusHandler serves the /status.html response which is always 200.
func statusHandler(rw http.ResponseWriter, req *http.Request) {
	log.Infof("Serving %s %s request for client: %s", req.Method, req.URL.Path, req.RemoteAddr)
	io.WriteString(rw, "OK")
}
