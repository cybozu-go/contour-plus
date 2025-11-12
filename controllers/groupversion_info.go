package controllers

import "k8s.io/apimachinery/pkg/runtime/schema"

var (
	// externalDNSGroupVersion is external-dns group version which is used to uniquely identifies the API
	externalDNSGroupVersion = schema.GroupVersion{Group: "externaldns.k8s.io", Version: "v1alpha1"}
	// certManagerGroupVersion is cert-manager group version which is used to uniquely identifies the API
	certManagerGroupVersion = schema.GroupVersion{Group: "cert-manager.io", Version: "v1"}
	//  contourGroupVersion is the contour group version which is used to uniquely identify the API
	contourGroupVersion = schema.GroupVersion{Group: "projectcontour.io", Version: "v1"}
)
