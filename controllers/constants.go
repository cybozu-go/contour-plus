package controllers

// Constants for Kinds
const (
	ClusterIssuerKind   = "ClusterIssuer"
	IssuerKind          = "Issuer"
	CertificateKind     = "Certificate"
	CertificateListKind = "CertificateList"
	DNSEndpointKind     = "DNSEndpoint"
	DNSEndpointListKind = "DNSEndpointList"
)

// Constants for certificate usages
const (
	usageDigitalSignature = "digital signature"
	usageKeyEncipherment  = "key encipherment"
	usageServerAuth       = "server auth"
)
