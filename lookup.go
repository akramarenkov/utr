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

//nolint:gochecknoglobals // For convenience, the variable itself is thread-safe.
var defaultKeeper Keeper = &MapKeeper{}

// Adds mapping of hostname and path to Unix socket using global [Keeper].
func AddPath(hostname, path string) error {
	return defaultKeeper.AddPath(hostname, path)
}

// Resolves path to Unix socket by hostname using global [Keeper].
func LookupPath(hostname string) (string, error) {
	return defaultKeeper.LookupPath(hostname)
}
