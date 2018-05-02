package dbconfig

import (
	"flag"
	"io/ioutil"
	"net/url"

	"github.com/pkg/errors"
)

// Config captures command line arguments pertaining to database access & migration
type Config struct {
	uri           string
	migrationsDir string
	passwordFile  string

	// TODO remove once all manifests updated to use new standard arguments
	uri2           string
	uri3           string
	uri4           string
	migrationsDir2 string
	migrationsDir3 string
}

// New makes a new config, primarily for testing
func New(uri, migrationsDir, passwordFile string) Config {
	return Config{uri: uri,
		migrationsDir: migrationsDir,
		passwordFile:  passwordFile}
}

// RegisterFlags registers configuration variables with a flag set
func (cfg *Config) RegisterFlags(f *flag.FlagSet, defaultURI, uriHelp, defaultMigrationsDir, migrationsDirHelp string) {
	f.StringVar(&cfg.uri, "database.uri", defaultURI, uriHelp)
	f.StringVar(&cfg.migrationsDir, "database.migrations", defaultMigrationsDir, migrationsDirHelp)
	f.StringVar(&cfg.passwordFile, "database.password-file", "", "File containing password (username goes in URI)")

	// TODO remove once all manifests updated to use new standard arguments
	f.StringVar(&cfg.uri2, "database-source", "", "Deprecated; use --database.uri")
	f.StringVar(&cfg.uri3, "database-uri", "", "Deprecated; use --database.uri")
	f.StringVar(&cfg.uri4, "db.uri", "", "Deprecated; use --database.uri")
	f.StringVar(&cfg.migrationsDir2, "database-migrations", "", "Deprecated; use --database.migrations")
	f.StringVar(&cfg.migrationsDir3, "db.migrations", "", "Deprecated; use --database.migrations")
}

// Parameters validates the database configuration arguments and reads in password material from the filsystem
func (cfg *Config) Parameters() (scheme, dataSourceName, migrationDir string, err error) {

	///////////////////////////////////////////////////////////////////////
	// TODO remove once all manifests updated to use new standard arguments
	switch {
	case len(cfg.uri2) > 0:
		cfg.uri = cfg.uri2
	case len(cfg.uri3) > 0:
		cfg.uri = cfg.uri3
	case len(cfg.uri4) > 0:
		cfg.uri = cfg.uri4
	}

	switch {
	case len(cfg.migrationsDir2) > 0:
		cfg.migrationsDir = cfg.migrationsDir2
	case len(cfg.migrationsDir3) > 0:
		cfg.migrationsDir = cfg.migrationsDir3
	}
	///////////////////////////////////////////////////////////////////////

	uri, err := url.Parse(cfg.uri)
	if err != nil {
		return "", "", "", errors.Wrap(err, "Could not parse database URI")
	}

	if len(cfg.passwordFile) != 0 {
		if uri.User == nil {
			return "", "", "", errors.New("--database.password-file requires username in --database.uri")
		}
		passwordBytes, err := ioutil.ReadFile(cfg.passwordFile)
		if err != nil {
			return "", "", "", errors.Wrap(err, "Could not read database password file")
		}
		uri.User = url.UserPassword(uri.User.Username(), string(passwordBytes))
	}

	return uri.Scheme, uri.String(), cfg.migrationsDir, nil
}
