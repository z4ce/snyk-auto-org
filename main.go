package main

import (
	"fmt"
	"os"

	"github.com/z4ce/snyk-auto-org/internal/app"
)

func main() {
	if err := app.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
