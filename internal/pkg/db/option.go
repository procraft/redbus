package db

type Option = func(d *DBPool)

func WithPoolSize(size int) Option {
	return func(d *DBPool) {
		d.size = size
	}
}

func WithLog() Option {
	return func(d *DBPool) {
		d.log = true
	}
}

// WithSSL sets the libpq sslmode and, optionally, the CA certificate for verify-ca / verify-full.
func WithSSL(mode, rootCert string) Option {
	return func(d *DBPool) {
		d.sslMode = mode
		d.sslRootCert = rootCert
	}
}
