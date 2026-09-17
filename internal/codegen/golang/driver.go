package golang

import "github.com/sqlc-dev/sqlc/internal/codegen/golang/opts"

func parseDriver(options *opts.Options) opts.SQLDriver {
	switch options.SqlPackage {
	case opts.SQLPackagePGXV4:
		return opts.SQLDriverPGXV4
	case opts.SQLPackagePGXV5:
		return opts.SQLDriverPGXV5
	}
	if options.SqlDriver == opts.SQLDriverDuckDB {
		return opts.SQLDriverDuckDB
	}
	return opts.SQLDriverLibPQ
}
