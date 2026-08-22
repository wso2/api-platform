package netguard

// PermitPrivateBlockMetadata returns the policy appropriate for calls to an
// operator- or tenant-configured backend that is normally *meant* to be
// private — a Kubernetes ClusterIP, a service-DNS name resolving into RFC
// 1918 space, or a localhost port during development. Private, loopback, and
// carrier-grade-NAT addresses are all permitted so ordinary deployment
// shapes keep working.
//
// What stays refused is the set of addresses that is never a legitimate
// upstream but is a standard SSRF target or a hazard: link-local addresses
// (which is where the cloud instance metadata endpoint 169.254.169.254
// lives), the unspecified address (which the OS can reinterpret as "local
// host"), and multicast/broadcast addresses.
func PermitPrivateBlockMetadata() Policy {
	return Policy{
		BlockLinkLocal:          true,
		BlockUnspecified:        true,
		BlockMulticastBroadcast: true,
	}
}

// PublicOnly returns the stricter policy appropriate for fetching a URL that
// is expected to point at the public internet (a vendor endpoint, a
// third-party spec URL) — every private, loopback, link-local,
// carrier-grade-NAT, unspecified, and multicast/broadcast address is
// refused.
func PublicOnly() Policy {
	return Policy{
		BlockPrivate:            true,
		BlockLoopback:           true,
		BlockLinkLocal:          true,
		BlockUnspecified:        true,
		BlockMulticastBroadcast: true,
		BlockCGNAT:              true,
	}
}
