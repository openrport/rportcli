package cmd

import (
	"bufio"
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/cloudradar-monitoring/rportcli/internal/pkg/output"

	"github.com/cloudradar-monitoring/rportcli/internal/pkg/controllers"

	"github.com/cloudradar-monitoring/rportcli/internal/pkg/api"
	"github.com/cloudradar-monitoring/rportcli/internal/pkg/config"
	"github.com/cloudradar-monitoring/rportcli/internal/pkg/utils"
	"github.com/spf13/cobra"
)

func init() {
	config.DefineCommandInputs(commandsCmd, getCommandRequirements())
	rootCmd.AddCommand(commandsCmd)
}

var commandsCmd = &cobra.Command{
	Use:   "command",
	Short: "executes remote command on rport client",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		baseRportURL := config.Params.ReadString(config.ServerURL, config.DefaultServerURL)
		wsURLBuilder := &api.WsCommandURLProvider{
			TokenProvider: func() (token string, err error) {
				token = config.Params.ReadString(config.Token, "")
				return
			},
			BaseURL: baseRportURL,
		}
		wsClient, err := utils.NewWsClient(ctx, wsURLBuilder.BuildWsURL)
		if err != nil {
			return err
		}

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		promptReader := &utils.PromptReader{
			Sc:              bufio.NewScanner(os.Stdin),
			SigChan:         sigs,
			PasswordScanner: utils.ReadPassword,
		}
		params, err := config.CollectParams(cmd, getCommandRequirements(), promptReader)
		if err != nil {
			return err
		}

		isFullJobOutput := params.ReadBool(controllers.IsFullOutput, false)
		cmdExecutor := &controllers.InteractiveCommandsController{
			ReadWriter: wsClient,
			JobRenderer: &output.JobRenderer{
				Writer:       os.Stdout,
				Format:       getOutputFormat(),
				IsFullOutput: isFullJobOutput,
			},
		}

		err = cmdExecutor.Start(ctx, params)

		return err
	},
}

func getCommandRequirements() []config.ParameterRequirement {
	return []config.ParameterRequirement{
		{
			Field:       controllers.ClientIDs,
			Help:        "Enter comma separated client IDs",
			Validate:    config.RequiredValidate,
			Description: "Comma separated client ids for which the command should be executed",
			ShortName:   "d",
			IsRequired:  true,
		},
		{
			Field:       controllers.Command,
			Help:        "Enter command",
			Validate:    config.RequiredValidate,
			Description: "Command which should be executed on the clients",
			ShortName:   "c",
			IsRequired:  true,
		},
		{
			Field:       controllers.Timeout,
			Help:        "Enter timeout in seconds",
			Description: "timeout in seconds that was used to observe the command execution",
			Default:     strconv.Itoa(controllers.DefaultCmdTimeoutSeconds),
			ShortName:   "t",
		},
		{
			Field:       controllers.GroupIDs,
			Help:        "Enter comma separated group IDs",
			Description: "Comma separated client group IDs",
			ShortName:   "g",
		},
		{
			Field:       controllers.ExecConcurrently,
			Help:        "execute the command concurrently on multiple clients",
			Description: "execute the command concurrently on multiple clients",
			ShortName:   "r",
			Type:        config.BoolRequirementType,
			Default:     "0",
		},
		{
			Field:       controllers.IsFullOutput,
			Help:        "output detailed information of a job execution",
			Description: "output detailed information of a job execution",
			ShortName:   "f",
			Type:        config.BoolRequirementType,
			Default:     "0",
		},
	}
}
