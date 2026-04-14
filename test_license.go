//go:build enterprise

package main

import (
	"fmt"
	"os"

	"github.com/coder/coder/v2/enterprise/coderd/license"
)

func main() {
	licenseString, claims, err := license.GenerateOfflineLicense(0) // 0 will use default 999999
	if err != nil {
		fmt.Printf("Error generating license: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("License: %s\n", licenseString)
	fmt.Printf("Claims: %+v\n", claims)
}
