package controllers

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	options "github.com/breathbath/go_utils/utils/config"
	"github.com/cloudradar-monitoring/rportcli/internal/pkg/config"
	"github.com/cloudradar-monitoring/rportcli/internal/pkg/models"
	"github.com/sirupsen/logrus"
)

const (
	defaultCmdTimeoutSeconds = 30
	clientIDs                = "cids"
	command                  = "command"
	groupIDs                 = "gids"
	timeout                  = "timeout"
	execConcurrently         = "conc"
	waitingMsg               = "waiting for the command to finish"
	finishedMsg              = "finished command execution"
)

func GetCommandRequirements() []config.ParameterRequirement {
	return []config.ParameterRequirement{
		{
			Field:       clientIDs,
			Help:        "Enter comma separated client IDs",
			Validate:    config.RequiredValidate,
			Description: "Comma separated client ids for which the command should be executed",
			ShortName:   "d",
			IsRequired:  true,
		},
		{
			Field:       command,
			Help:        "Enter command",
			Validate:    config.RequiredValidate,
			Description: "Command which should be executed on the clients",
			ShortName:   "c",
			IsRequired:  true,
		},
		{
			Field:       timeout,
			Help:        "Enter timeout in seconds",
			Description: "timeout in seconds that was used to observe the command execution",
			Default:     strconv.Itoa(defaultCmdTimeoutSeconds),
			ShortName:   "t",
		},
		{
			Field:       groupIDs,
			Help:        "Enter comma separated group IDs",
			Description: "Comma separated client group IDs",
			ShortName:   "g",
		},
		{
			Field:       execConcurrently,
			Help:        "execute the command concurrently on multiple clients",
			Description: "execute the command concurrently on multiple clients",
			ShortName:   "r",
		},
	}
}

type CliReader interface {
	ReadString() (string, error)
}

type ReadWriter interface {
	Read() (msg []byte, err error)
	Write(inputMsg []byte) (n int, err error)
	io.Closer
}

type Spinner interface {
	Start(msg string)
	Update(msg string)
	Stop(msg string)
}

type InteractiveCommandsController struct {
	ReadWriter   ReadWriter
	PromptReader config.PromptReader
	Spinner      Spinner
}

func (icm *InteractiveCommandsController) Start(ctx context.Context, parametersFromArguments map[string]*string) error {
	defer icm.ReadWriter.Close()

	params, err := icm.collectParams(parametersFromArguments)
	if err != nil {
		return err
	}

	wsCmd := icm.buildCommand(params)
	err = icm.sendCommand(wsCmd)
	if err != nil {
		return err
	}
	readingCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = icm.startReading(readingCtx)

	return err
}

func (icm *InteractiveCommandsController) collectParams(
	parametersFromArguments map[string]*string,
) (params *options.ParameterBag, err error) {
	paramsFromArguments := make(map[string]string, len(parametersFromArguments))
	for k, valP := range parametersFromArguments {
		paramsFromArguments[k] = *valP
	}
	params = config.FromValues(paramsFromArguments)
	missedRequirements := config.CheckRequirements(params, GetCommandRequirements())
	if len(missedRequirements) == 0 {
		return
	}
	err = config.PromptRequiredValues(missedRequirements, paramsFromArguments, icm.PromptReader)
	if err != nil {
		return
	}
	params = config.FromValues(paramsFromArguments)

	return
}

func (icm *InteractiveCommandsController) buildCommand(params *options.ParameterBag) models.WsCommand {
	wsCmd := models.WsCommand{
		Command:             params.ReadString(command, ""),
		ClientIds:           strings.Split(params.ReadString(clientIDs, ""), ","),
		TimeoutSec:          params.ReadInt(timeout, defaultCmdTimeoutSeconds),
		ExecuteConcurrently: params.ReadBool(execConcurrently, false),
		GroupIds:            nil,
	}
	groupIDsStr := params.ReadString(groupIDs, "")
	if groupIDsStr != "" {
		groupIDsList := strings.Split(groupIDsStr, ",")
		wsCmd.GroupIds = &groupIDsList
	}

	return wsCmd
}

func (icm *InteractiveCommandsController) sendCommand(wsCmd models.WsCommand) error {
	wsCmdJSON, err := json.Marshal(wsCmd)
	if err != nil {
		return err
	}
	logrus.Debugf("will send %s", string(wsCmdJSON))

	_, err = icm.ReadWriter.Write(wsCmdJSON)
	if err != nil {
		return err
	}

	return nil
}

func (icm *InteractiveCommandsController) startReading(ctx context.Context) error {
	errsChan := make(chan error, 1)
	msgChan := make(chan string, 1)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		defer close(msgChan)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := icm.ReadWriter.Read()
				if err != nil {
					if err == io.EOF {
						return
					}
					errsChan <- err
				}
				msgChan <- string(msg)
			}
		}
	}()

	icm.Spinner.Start(waitingMsg)
	defer icm.Spinner.Stop(finishedMsg)
mainLoop:
	for {
		select {
		case <-sigs:
			break mainLoop
		case msg, ok := <-msgChan:
			if !ok {
				return nil
			}
			err := icm.processMessage(msg)
			if err != nil {
				return err
			}
			icm.Spinner.Start(waitingMsg)
		case err := <-errsChan:
			return err
		}
	}

	return nil
}

func (icm *InteractiveCommandsController) processMessage(msg string) error {
	icm.Spinner.Stop(msg)
	return nil
}