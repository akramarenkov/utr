package utr

// Resolves path to Unix socket by hostname.
type Resolver interface {
	LookupPath(hostname string) (string, error)
}
