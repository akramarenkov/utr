package utr

// Provides adjusting of a Unix domain socket transport.
type Adjuster interface {
	adjust(t *Transport) error
}

type adjust func(t *Transport) error

func (adj adjust) adjust(t *Transport) error {
	return adj(t)
}
