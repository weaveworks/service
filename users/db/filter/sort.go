package filter

import (
	"fmt"
)

type SortDir string

const (
	SortDesc SortDir = "desc"
	SortAsc  SortDir = "asc"
)

type Sort struct {
	column string
	dir    SortDir
}

func NewSort(column string, dir SortDir) Sort {
	return Sort{column, dir}
}

func (s Sort) OrderBy() string {
	return fmt.Sprintf("%s %s", s.column, s.dir)
}
