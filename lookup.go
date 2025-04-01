package utr

// Collects mappings of hostnames and paths to Unix sockets.
type Collector interface {
	AddPath(hostname, path string) error
}

// Resolves path to Unix socket by hostname.
type Resolver interface {
	LookupPath(hostname string) (string, error)
}

// Combines [Collector] and [Resolver].
type Keeper interface {
	Collector
	Resolver
}
