package main

import (
	"flag"
	"os"
	"fmt"
)

func main() {
	fs := flag.NewFlagSet("subprocess", flag.PanicOnError)
	var tl TimeLimit
	var ml MemoryLimit

	fs.Var(&tl, "t", "time limit")
	fs.Var(&ml, "m", "memory limit")

	fs.Parse(os.Args[1:])

	fmt.Println(tl, ml)
}
