package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	hooks "github.com/containers/common/pkg/hooks/1.0.0"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/spf13/cobra"
)

var hookDir string
var criSocket string

// injectCmd represents the inject command to place the OCI hook configuration
var injectCmd = &cobra.Command{
	Use:   "inject",
	Short: "Inject OCI hooks",
	Long:  `Injects an OCI hook configuration for KubeArmor into the specified directory.`,
	RunE: func(cmd *cobra.Command, args []string) error {

		cleanHookDir := filepath.Clean(hookDir)
		if !filepath.IsAbs(cleanHookDir) {
			return errors.New("hook-dir must be an absolute path")
		}

		if err := os.MkdirAll(hookDir, 0750); err != nil {
			return err
		}

		dst, err := os.OpenFile(filepath.Join(cleanHookDir, "ka.json"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer dst.Close()

		always := true
		hook := hooks.Hook{
			Version: "1.0.0",
			Hook: specs.Hook{
				Path: "/usr/share/kubearmor/hook",
				Args: []string{
					"/usr/share/kubearmor/hook",
					"--runtime-socket",
					criSocket,
				},
				Env: []string{
					"XDG_CONFIG_HOME=/tmp/",
				},
			},
			When: hooks.When{Always: &always},
			Stages: []string{
				"poststart",
				"poststop",
			},
		}

		hookBytes, err := json.Marshal(hook)
		if err != nil {
			return err
		}

		_, err = dst.Write(hookBytes)
		if err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(injectCmd)

	injectCmd.Flags().StringVar(&hookDir, "hook-dir", "", "Path to the hook config directory (e.g., /usr/share/containers/oci/hooks.d)")
	injectCmd.Flags().StringVar(&criSocket, "cri-socket", "", "Path to the runtime socket (e.g., /run/podman/podman.sock)")
	if err := injectCmd.MarkFlagRequired("hook-dir"); err != nil {
		panic(err)
	}
	if err := injectCmd.MarkFlagRequired("cri-socket"); err != nil {
		panic(err)
	}
}
