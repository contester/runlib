package main

import (
	"flag"
	"path/filepath"
	"io/ioutil"
	"bytes"
)

func main() {
	flag.Parse()
	workdir := flag.Args()

	genAnswer := filepath.Join(workdir, "scripts", "gen-answer.bat")
	src, err := ioutil.ReadFile(genAnswer)
	if err != nil {
		return
	}

	dst := bytes.Replace(src, "java.exe -Xmx512M -Xss128M -DONLINE_JUDGE=true -Duser.language=en -Duser.region=US -Duser.variant=US -jar files/CrossRun.jar ", "files\runexe.exe -interactor=", 1)
	if dst != src {
		ioutil.WriteFile(genAnswer, dst, 0755)
	}
}
