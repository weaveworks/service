package filter_test

import (
	"github.com/Masterminds/squirrel"
	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/users/db/filter"
	"testing"
)

type mockFilter struct {
	key   string
	value string
}

func (f mockFilter) Where() squirrel.Sqlizer {
	return squirrel.Eq{f.key: f.value}
}
func (f mockFilter) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	return b.Where(f.Where())
}

var (
	fa = mockFilter{"a", "1"}
	fb = mockFilter{"b", "2"}
)

func TestOr(t *testing.T) {
	or := filter.Or(fa, fb)
	sql, _, err := or.Where().ToSql()
	assert.NoError(t, err)
	assert.Equal(t, "(a = ? OR b = ?)", sql)
}

func TestAnd(t *testing.T) {
	or := filter.And(fa, fb)
	sql, _, err := or.Where().ToSql()
	assert.NoError(t, err)
	assert.Equal(t, "(a = ? AND b = ?)", sql)
}

func TestAndOr(t *testing.T) {
	or := filter.And(filter.Or(fa, fb), fb)
	sql, _, err := or.Where().ToSql()
	assert.NoError(t, err)
	assert.Equal(t, "((a = ? OR b = ?) AND b = ?)", sql)
}
