package main

import (
	"os"

	"labs.lesiw.io/ops/goapp"
	"lesiw.io/ops"
)

type Ops struct{ goapp.Ops }

func main() {
	goapp.Name = "spkez"
	goapp.Targets = []goapp.Target{
		{Goos: "linux", Goarch: "386", Unames: "linux", Unamer: "i386"},
		{Goos: "linux", Goarch: "amd64", Unames: "linux", Unamer: "x86_64"},
		{Goos: "linux", Goarch: "arm", Unames: "linux", Unamer: "armv7l"},
		{Goos: "linux", Goarch: "arm64", Unames: "linux", Unamer: "aarch64"},
		{Goos: "darwin", Goarch: "amd64", Unames: "darwin", Unamer: "x86_64"},
		{Goos: "darwin", Goarch: "arm64", Unames: "darwin", Unamer: "arm64"},
		{Goos: "windows", Goarch: "386", Unames: "windows", Unamer: "i386"},
		{Goos: "windows", Goarch: "amd64", Unames: "windows", Unamer: "x86_64"},
	}
	if len(os.Args) < 2 {
		os.Args = append(os.Args, "build")
	}
	ops.Handle(Ops{})
}
