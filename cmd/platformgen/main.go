// Command platformgen generates independent Wire adapters for declared drivers.
package main

import (
	"flag"
	"fmt"
	"os"

	"gitlab.com/skyrix-lib/framework/scaffold/platform"
)

func main() {
	configPackage := flag.String("config", "", "application config package (default <module>/internal/config)")
	flag.Parse()
	dir, err := os.Getwd()
	if err == nil {
		err = platform.Generate(dir, *configPackage)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "platformgen:", err)
		os.Exit(1)
	}
}
