package main

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/common/errors"
)

// == with interfaces has some oddities, make sure the behavior is what we
// expect.
func TestErrorStatusCode(t *testing.T) {
	tests := []struct {
		err      error
		expected int
	}{
		{errors.ErrNotFound, http.StatusNoContent},
		{errors.ErrInvalidParameter, http.StatusBadRequest},
		{fmt.Errorf("foo"), http.StatusInternalServerError},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, errorStatusCode(test.err))
	}
}
