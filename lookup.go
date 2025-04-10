package utr

// Resolves path to Unix domain socket by hostname.
type Resolver interface {
	LookupPath(hostname string) (string, error)
}
