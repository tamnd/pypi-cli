package cli

import (
	"errors"

	"github.com/tamnd/pypi-cli/pypi"
)

func isNotFound(err error) bool {
	return errors.Is(err, pypi.ErrNotFound)
}
