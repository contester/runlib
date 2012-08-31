// +build windows,386

package service

import (
"syscall"
)

func OnOsCreateError(err error) (bool, error) {
                if err != nil {
                        l4g.Error(err)
                        if err == syscall.ERROR_ACCESS_DENIED {
                                return true, nil
                        }
                        return false, err
                }
                return false, nil
}
