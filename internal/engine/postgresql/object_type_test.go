package postgresql

import (
	"testing"

	pg "github.com/sqlc-dev/oliphant"

	"github.com/sqlc-dev/sqlc/internal/sql/ast"
)

// TestObjectTypeMatview pins the value core/schema reads a materialized
// view by to the one the parser reports.
func TestObjectTypeMatview(t *testing.T) {
	if got := ast.ObjectType(pg.ObjectType_OBJECT_MATVIEW); got != ast.ObjectTypeMatview {
		t.Fatalf("ast.ObjectTypeMatview is %d, the parser reports %d", ast.ObjectTypeMatview, got)
	}
}
