package globals

import (
	"errors"
	"fmt"
)

type Globals map[string]any

func (g Globals) Merge(other Globals) (res Globals, err error) {
	res = Globals{}

	for k, v := range g {
		res[k] = v
	}

	errs := []error{}

	for k, v := range other {
		_, contains := res[k]
		if contains {
			errs = append(errs, fmt.Errorf("could not merge globals, %s is set in both globals", k))
			continue
		}
		res[k] = v
	}

	if len(errs) != 0 {
		return nil, errors.Join(errs...)
	}

	return res, nil

}
