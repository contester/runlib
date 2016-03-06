package service

import (
	"syscall"

	log "github.com/Sirupsen/logrus"
)

func OnOsCreateError(err error) (bool, error) {
	if err != nil {
		log.Error(err)
		if err == syscall.ERROR_ACCESS_DENIED {
			return true, nil
		}
		return false, err
	}
	return false, nil
}
