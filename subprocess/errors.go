package subprocess

import "github.com/contester/runlib/tools"

const ERR_USER = "HANDS"

func IsUserError(err error) bool {
	return tools.HasAnnotation(err, ERR_USER)
}
