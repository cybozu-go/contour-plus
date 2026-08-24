package controllers

// Constants for Kinds
const (
	ClusterIssuerKind                = "ClusterIssuer"
	IssuerKind                       = "Issuer"
	CertificateKind                  = "Certificate"
	CertificateListKind              = "CertificateList"
	DNSEndpointKind                  = "DNSEndpoint"
	DNSEndpointListKind              = "DNSEndpointList"
	TLSCertificateDelegationKind     = "TLSCertificateDelegation"
	TLSCertificateDelegationListKind = "TLSCertificateDelegationList"
)

// Constants for certificate usages
const (
	usageDigitalSignature = "digital signature"
	usageKeyEncipherment  = "key encipherment"
	usageServerAuth       = "server auth"
)

// Constants for TTL values
const (
	DefaultDNSTTL           = 3600
	DefaultDNSDelegationTTL = 60
)
