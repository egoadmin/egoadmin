package permission

import "strings"

// Policy represents one gRPC service/method permission rule.
type Policy struct {
	Service string
	Method  string
}

// NormalizePolicies uppercases fields, removes empty entries, and de-duplicates policies.
func NormalizePolicies(policies []Policy) []Policy {
	normalized := make([]Policy, 0, len(policies))
	seen := make(map[Policy]struct{}, len(policies))
	for _, policy := range policies {
		policy = Policy{
			Service: strings.ToUpper(strings.TrimSpace(policy.Service)),
			Method:  strings.ToUpper(strings.TrimSpace(policy.Method)),
		}
		if policy.Service == "" || policy.Method == "" {
			continue
		}
		if _, ok := seen[policy]; ok {
			continue
		}
		seen[policy] = struct{}{}
		normalized = append(normalized, policy)
	}
	return normalized
}

// FullPath returns the normalized API identity for a policy.
func (p Policy) FullPath() string {
	return strings.ToUpper(strings.TrimSpace(p.Service) + "/" + strings.TrimSpace(p.Method))
}
