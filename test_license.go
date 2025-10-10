//go:build enterprise

package main

import (
	"fmt"
	"os"

	"github.com/coder/coder/v2/enterprise/coderd/license"
)

func main() {
	licenseString, claims, err := license.GenerateOfflineLicense()
	if err != nil {
		fmt.Printf("Error generating license: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("License: %s\n", licenseString)
	fmt.Printf("Claims: %+v\n", claims)
}