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
	UsageDigitalSignature = "digital signature"
	UsageKeyEncipherment  = "key encipherment"
	UsageServerAuth       = "server auth"
	UsageClientAuth       = "client auth"
)
