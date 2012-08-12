package main

/*
#include <stdio.h>

int f(int x) { printf("%d\n", x); return 1; }
*/
import "C"

import (
  "fmt"
)

func main() {
  fmt.Printf("palevo %d", int(C.f(100)))

  
}