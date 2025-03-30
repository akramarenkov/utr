package utr

// Collects mappings of names and paths to Unix sockets.
type Collector interface {
	AddPath(host, path string) error
}

// Resolves path to Unix socket by name.
type Resolver interface {
	LookupPath(host string) (string, error)
}

// Combines [Collector] and [Resolver].
type Keeper interface {
	Collector
	Resolver
}

//nolint:gochecknoglobals // For convenience, the variable itself is thread-safe.
var defaultKeeper Keeper = &MapKeeper{}

// Adds mapping of name and path to Unix socket using package-wide [Keeper].
func AddPath(host, path string) error {
	return defaultKeeper.AddPath(host, path)
}

// Resolves path to Unix socket by name using package-wide [Keeper].
func LookupPath(host string) (string, error) {
	return defaultKeeper.LookupPath(host)
}
