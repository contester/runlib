// +build linux

package service

import (
)

func OnOsCreateError(err error) (bool, error) {
	return false, err
}
