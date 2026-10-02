//go:build !unix

package prop

import "errors"

func (local) Setprop(name, value string) error {
	return errors.New("prop: setprop needs an Android device")
}
