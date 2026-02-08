package main

import (
	"os"

	"labs.lesiw.io/ops/goapp"
	"lesiw.io/ops"
)

func main() {
	goapp.Name = "spkez"
	goapp.Targets = append(goapp.Targets,
		goapp.Target{
			Goos: "windows", Goarch: "386",
			Unames: "windows", Unamer: "i386",
		},
		goapp.Target{
			Goos: "windows", Goarch: "amd64",
			Unames: "windows", Unamer: "x86_64",
		},
	)
	if len(os.Args) < 2 {
		os.Args = append(os.Args, "check")
	}
	ops.Handle(goapp.Ops{})
}
