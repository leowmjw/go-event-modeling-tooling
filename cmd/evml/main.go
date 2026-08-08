package main

import (
	"os"

	"github.com/leowmjw/go-event-modeling-tooling"
)

func main() {
	os.Exit(evml.Run(os.Args[1:], os.Stdout, os.Stderr))
}
