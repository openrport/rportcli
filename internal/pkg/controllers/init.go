package controllers

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"

	options "github.com/breathbath/go_utils/v2/pkg/config"
	"github.com/breathbath/go_utils/v2/pkg/env"
	"github.com/cloudradar-monitoring/rportcli/internal/pkg/api"
	"github.com/cloudradar-monitoring/rportcli/internal/pkg/auth"
	"github.com/cloudradar-monitoring/rportcli/internal/pkg/config"
	"github.com/cloudradar-monitoring/rportcli/internal/pkg/models"
	"github.com/cloudradar-monitoring/rportcli/internal/pkg/utils"
	"github.com/pkg/errors"
)

var (
	ErrNoPromptInUseWhenTwoFA = errors.New("2fa requested when using --no-prompt. Please retry without the --no-prompt option")
	ErrNoPromptInUseWhenTotP  = errors.New("totP requested when using --no-prompt. Please retry without the --no-prompt option")
)

type ConfigWriter func(params *options.ParameterBag) (err error)

type QrImageWriterProvider func(namePattern string) (writer io.Writer, closr io.Closer, name string, err error)

type TotPSecretRenderer interface {
	RenderTotPSecret(key *models.TotPSecretOutput) error
}

type InitController struct {
	ConfigWriter          ConfigWriter
	PromptReader          config.PromptReader
	TotPSecretRenderer    TotPSecretRenderer
	QrImageWriterProvider QrImageWriterProvider
}

func (ic *InitController) InitConfig(ctx context.Context, params *options.ParameterBag) error {
	login := config.ReadAPIUser(params)
	serverURL := config.ReadAPIURL(params)

	apiAuth := &utils.StorageBasicAuth{
		AuthProvider: func() (l, p string, err error) {
			return auth.GetUsernameAndPassword(params)
		},
	}

	tokenValidity := env.ReadEnvInt(config.SessionValiditySecondsEnvVar, api.DefaultTokenValiditySeconds)

	cl := api.New(serverURL, apiAuth)
	loginResp, err := cl.GetToken(ctx, tokenValidity)
	if err != nil {
		return fmt.Errorf("config verification failed: %v", err)
	}

	if loginResp.Data.Token == "" {
		return fmt.Errorf("no auth token received from rport")
	}

	noPrompt := params.ReadBool(config.NoPrompt, false)
	if loginResp.Data.TwoFA.DeliveryMethod == "totp_authenticator_app" {
		if noPrompt {
			return ErrNoPromptInUseWhenTotP
		}
		cl := api.New(serverURL, &utils.BearerAuth{
			TokenProvider: func() (string, error) {
				return loginResp.Data.Token, nil
			},
		})
		loginResp, err = ic.processTotP(ctx, loginResp.Data.TwoFA.TotPKeyStatus, cl, login, tokenValidity)
		if err != nil {
			return fmt.Errorf("totP secret processing to rport failed: %v", err)
		}
	}

	if loginResp.Data.TwoFA.SentTo != "" {
		if noPrompt {
			return ErrNoPromptInUseWhenTwoFA
		}

		cl := api.New(serverURL, &utils.BearerAuth{
			TokenProvider: func() (string, error) {
				return loginResp.Data.Token, nil
			},
		})
		loginResp, err = ic.process2FA(ctx, cl, loginResp.Data, login, tokenValidity)
		if err != nil {
			return fmt.Errorf("2 factor login to rport failed: %v", err)
		}
	}

	if loginResp.Data.Token == "" {
		return fmt.Errorf("no auth token received from rport")
	}

	valuesProvider := options.NewMapValuesProvider(map[string]interface{}{
		config.ServerURL: config.ReadAPIURLWithDefault(params, ""),
		config.Token:     loginResp.Data.Token,
	})

	err = ic.ConfigWriter(options.New(valuesProvider))
	if err != nil {
		return err
	}

	return nil
}

func (ic *InitController) process2FA(
	ctx context.Context,
	cl *api.Rport,
	loginToken models.Token,
	username string,
	tokenLifetime int,
) (li api.LoginResponse, err error) {
	req := config.ParameterRequirement{
		Field: "code",
		Help: fmt.Sprintf(
			"2 factor auth is enabled, please provide code that was sent to %s via %s",
			loginToken.TwoFA.SentTo,
			loginToken.TwoFA.DeliveryMethod,
		),
		Validate:   config.RequiredValidate,
		IsRequired: true,
		Type:       config.StringRequirementType,
	}
	resultMap := map[string]interface{}{}
	err = config.PromptRequiredValues([]config.ParameterRequirement{req}, resultMap, ic.PromptReader)
	if err != nil {
		return li, err
	}

	li, err = cl.GetTokenBy2FA(ctx, resultMap["code"].(string), username, tokenLifetime)

	return li, err
}

func (ic *InitController) processTotP(
	ctx context.Context,
	totPSecretKeyStatus string,
	cl *api.Rport,
	login string,
	tokenLifetime int,
) (li api.LoginResponse, err error) {
	if totPSecretKeyStatus == api.TotPKeyPending {
		var totpSecretResp *models.TotPSecretResp
		totpSecretResp, err = cl.CreateTotPSecret(ctx)
		if err != nil {
			return li, err
		}

		totPSecretOutput := &models.TotPSecretOutput{
			Secret: totpSecretResp.Secret,
		}
		totPSecretOutput.File, err = ic.saveTotPSecretQr(totpSecretResp.QRImageBase64)
		if err != nil {
			return li, err
		}

		totPSecretOutput.Comment = "New Authenticator app secret key was created. " +
			"Please use the secret key below to create a new account in an Authenticator app. " +
			"Alternatively you can open the qr code image and scan it with your camera. " +
			"Don't forget to delete the qr code image after it!"

		err = ic.TotPSecretRenderer.RenderTotPSecret(totPSecretOutput)
		if err != nil {
			return li, err
		}
	}

	req := config.ParameterRequirement{
		Field:      "code",
		Help:       "Please provide code generated by your Authenticator app",
		Validate:   config.RequiredValidate,
		IsRequired: true,
		Type:       config.StringRequirementType,
	}
	resultMap := map[string]interface{}{}
	err = config.PromptRequiredValues([]config.ParameterRequirement{req}, resultMap, ic.PromptReader)
	if err != nil {
		return li, err
	}

	li, err = cl.GetTokenBy2FA(ctx, resultMap["code"].(string), login, tokenLifetime)

	return li, err
}

func (ic *InitController) saveTotPSecretQr(qrBase64 string) (filePath string, err error) {
	qrWriter, clsr, name, err := ic.QrImageWriterProvider("qr-*.png")

	if err != nil {
		return "", err
	}
	if clsr != nil {
		defer clsr.Close()
	}

	decoder := base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(qrBase64))

	_, err = io.Copy(qrWriter, decoder)
	if err != nil {
		return "", errors.Wrapf(err, "failed to decode %s from base64", qrBase64)
	}

	return name, nil
}
