// +build linux

package service

import (
	"code.google.com/p/goconf/conf"
	"strconv"
)

const PLATFORM_ID = "linux"

func OnOsCreateError(err error) (bool, error) {
	return false, err
}

func getPasswords(c *conf.ConfigFile) ([]string, error) {
	count, err := c.GetInt("default", "sandboxCount")
	if err != nil {
		return nil, err
	}
	result := make([]string, count)
	for i := range result {
		result[i] = "password" + strconv.Itoa(i)
	}
	return result, nil
}

func setAcl(path, username string) error {
	// TODO: use setfacl to set acl
	return nil
}
