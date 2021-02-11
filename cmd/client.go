package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudradar-monitoring/rportcli/internal/pkg/output"

	"github.com/cloudradar-monitoring/rportcli/internal/pkg/controllers"
	"github.com/spf13/cobra"
)

func init() {
	clientsCmd.AddCommand(clientsListCmd)
	clientsCmd.AddCommand(clientCmd)
	rootCmd.AddCommand(clientsCmd)
}

var clientsCmd = &cobra.Command{
	Use:   "client [command]",
	Short: "Client API",
	Args:  cobra.ArbitraryArgs,
}

var clientsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Client List API",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		rportAPI, err := buildRport()
		if err != nil {
			return err
		}
		cr := &output.ClientRenderer{}
		clientsController := &controllers.ClientController{
			Rport:          rportAPI,
			ClientRenderer: cr,
		}

		return clientsController.Clients(context.Background(), os.Stdout)
	},
}

var clientCmd = &cobra.Command{
	Use:   "get <ID>",
	Short: "Client Read API",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("client id is not provided")
		}

		rportAPI, err := buildRport()
		if err != nil {
			return err
		}

		cr := &output.ClientRenderer{}
		clientsController := &controllers.ClientController{
			Rport:          rportAPI,
			ClientRenderer: cr,
		}

		return clientsController.Client(context.Background(), args[0], os.Stdout)
	},
}
