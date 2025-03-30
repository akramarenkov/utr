package utr

type Compiler interface {
	AddPath(host, path string) error
}

type Resolver interface {
	LookupPath(host string) (string, error)
}

type Keeper interface {
	Compiler
	Resolver
}

//nolint:gochecknoglobals // For convenience, the variable itself is thread-safe.
var defaultKeeper Keeper = &MapKeeper{}

func AddPath(host, path string) error {
	return defaultKeeper.AddPath(host, path)
}

func LookupPath(host string) (string, error) {
	return defaultKeeper.LookupPath(host)
}
