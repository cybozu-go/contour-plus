package controllers

// Constants for Kinds
const (
	ClusterIssuerKind = "ClusterIssuer"
	IssuerKind        = "Issuer"
	CertificateKind   = "Certificate"
	DNSEndpointKind   = "DNSEndpoint"
)

// Constants for certificate usages
const (
	usageDigitalSignature = "digital signature"
	usageKeyEncipherment  = "key encipherment"
	usageServerAuth       = "server auth"
	usageClientAuth       = "client auth"
)
