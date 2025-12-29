//go:build !slim

package cli

import (
	"fmt"
	"os"

	"github.com/coder/coder/v2/enterprise/coderd/license"
	"github.com/coder/serpent"
	"gvisor.dev/gvisor/pkg/log"
)

func (r *RootCmd) generateLicense() *serpent.Command {
	var (
		outputFile     string
		publicKeyFile  string
		privateKeyFile string
	)

	cmd := &serpent.Command{
		Use:   "generate-license",
		Short: "Generate an offline license for Coder",
		Long:  "Generate a 10-year offline license for use in air-gapped environments.",
		Handler: func(inv *serpent.Invocation) error {
			fmt.Fprintf(inv.Stderr, "Generating offline license...\n")
			licenseString, _, _ := license.GenerateOfflineLicense()
			// 验证生成的license可以被正确解析
			_, err := license.ParseClaims(licenseString, license.GetOfflineKeys())
			if err != nil {
				return fmt.Errorf("generated license cannot be parsed: %w", err)
			}

			log.Infof("publicKeyFile: %s, privateKeyFile: %s, outputFile: %s", publicKeyFile, privateKeyFile, outputFile)

			if publicKeyFile != "" {
				// 将公钥写入文件
				err := license.SaveOfflinePublicKeyToFile(publicKeyFile)
				if err != nil {
					return fmt.Errorf("failed to write public key to file: %w", err)
				}
				fmt.Fprintf(inv.Stderr, "Public key written to %s\n", publicKeyFile)
			}

			if privateKeyFile != "" {
				err := license.SaveOfflinePrivateKeyToFile(privateKeyFile)
				if err != nil {
					return fmt.Errorf("failed to write private key to file: %w", err)
				}
				fmt.Fprintf(inv.Stderr, "Private key written to %s\n", privateKeyFile)
			}

			if outputFile == "" {
				// 输出到标准输出
				fmt.Fprintln(inv.Stdout, licenseString)
			} else {
				// 输出到文件
				err := os.WriteFile(outputFile, []byte(licenseString), 0644)
				if err != nil {
					return fmt.Errorf("failed to write license to file: %w", err)
				}
				fmt.Fprintf(inv.Stderr, "License written to %s\n", outputFile)
			}

			return nil
		},
	}

	cmd.Options = serpent.OptionSet{
		{
			Flag:          "output",
			FlagShorthand: "o",
			Env:           "CODER_LICENSE_OUTPUT",
			Description:   "Output file path for the license. If not specified, prints to stdout.",
			Value:         serpent.StringOf(&outputFile),
		},
		{
			Flag:          "public-key",
			FlagShorthand: "k",
			Env:           "CODER_LICENSE_PUBLIC_KEY_FILE",
			Description:   "Output file path for the public key. If specified, also saves the private key with a .private suffix.",
			Value:         serpent.StringOf(&publicKeyFile),
		},
		{
			Flag:          "private-key",
			FlagShorthand: "p",
			Env:           "CODER_LICENSE_PRIVATE_KEY_FILE",
			Description:   "Output file path for the private key. If specified, also saves the private key with a .private suffix.",
			Value:         serpent.StringOf(&privateKeyFile),
		},
	}

	return cmd
}
