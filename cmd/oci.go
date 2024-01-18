// SPDX-License-Identifier: Apache-2.0
// Copyright 2023 Authors of KubeArmor

package cmd

import (
	"fmt"

	"github.com/kubearmor/kubearmor-client/oci"
	"github.com/spf13/cobra"
)

type ociOptions struct {
	Image        string
	Policies     []string
	Output       string
	Username     string
	Password     string
	SkipValidate bool
}

var ociOpt ociOptions

// ociCmd represents the oci command
var ociCmd = &cobra.Command{
	Use:   "oci",
	Short: "Interact with OCI registry for KubeArmor policies",
	Long:  `Interact with OCI registry for managing KubeArmor policies.`,
}

// ociPushCmd represents oci push command
var ociPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push KubeArmor policies to OCI registry",
	Long: `Push KubeArmor policies to OCI registry.

Examples:
  # TODO(akshay): Add examples here.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		or := oci.New(ociOpt.Image, ociOpt.Policies,
			ociOpt.Username, ociOpt.Password)
		if !ociOpt.SkipValidate {
			msg, valid, err := or.ValidatePolicies()
			if err != nil {
				return err
			}
			if !valid {
				return fmt.Errorf("Policy validation failed (use --skip-validate to skip validation):\n %s", msg)
			}
		}
		if err := or.Push(); err != nil {
			return err
		}
		return nil
	},
}

// ociPullCmd represents oci pull command
var ociPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull image with KubeArmor policies from OCI registry",
	Long: `Pull image with KubeArmor policies from OCI registry.

Examples:
  # TODO(akshay): Add examples here.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		or := oci.New(ociOpt.Image, ociOpt.Policies, ociOpt.Username, ociOpt.Password)
		if err := or.Pull(ociOpt.Output); err != nil {
			return err
		}
		if !ociOpt.SkipValidate {
			msg, valid, err := or.ValidatePolicies()
			if err != nil {
				return err
			}
			if !valid {
				return fmt.Errorf("Policy validation failed (use --skip-validate to skip validation):\n %s", msg)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(ociCmd)

	ociCmd.PersistentFlags().StringVarP(&ociOpt.Image, "image", "i", "", "image that push or pull (required)")
	ociCmd.MarkPersistentFlagRequired("image")
	ociCmd.PersistentFlags().StringVarP(&ociOpt.Username, "username", "u", "", "OCI registry username")
	ociCmd.PersistentFlags().StringVarP(&ociOpt.Password, "password", "p", "", "OCI registry password")
	ociCmd.MarkFlagsRequiredTogether("username", "password")
	ociCmd.PersistentFlags().BoolVar(&ociOpt.SkipValidate, "skip-validate", false, "Skips KubeArmor policy validation")

	// Push command
	ociCmd.AddCommand(ociPushCmd)
	ociPushCmd.Flags().StringArrayVarP(&ociOpt.Policies, "policy", "f", []string{}, "KubeArmor policy file path (YAML format)")

	// Pull command
	ociCmd.AddCommand(ociPullCmd)
	ociPullCmd.Flags().StringVarP(&ociOpt.Output, "output", "o", "", "Location for storing KubeArmor policy files")
}
