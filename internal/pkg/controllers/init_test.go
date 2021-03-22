package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/cloudradar-monitoring/rportcli/internal/pkg/config"
	"github.com/cloudradar-monitoring/rportcli/internal/pkg/models"
	"github.com/sirupsen/logrus"

	options "github.com/breathbath/go_utils/v2/pkg/config"
	"github.com/cloudradar-monitoring/rportcli/internal/pkg/api"
	"github.com/stretchr/testify/assert"
)

func TestInitSuccess(t *testing.T) {
	statusRequested := false

	const tokenGiven = "some token"
	const tokenValidityVal = "90"
	err := os.Setenv(config.SessionValiditySecondsEnvVar, tokenValidityVal)
	assert.NoError(t, err)
	if err == nil {
		defer func() {
			e := os.Unsetenv(config.SessionValiditySecondsEnvVar)
			if e != nil {
				logrus.Error(e)
			}
		}()
	}

	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		resp := api.LoginResponse{
			Data: models.Token{
				Token: tokenGiven,
			},
		}
		statusRequested = true
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/login?token-lifetime="+tokenValidityVal, r.URL.String())
		assert.Equal(t, "Basic bG9naW46cGFzc3dvcmRz", r.Header.Get("Authorization"))

		rw.WriteHeader(http.StatusOK)
		jsonEnc := json.NewEncoder(rw)
		e := jsonEnc.Encode(resp)
		assert.NoError(t, e)
	}))
	defer srv.Close()

	writtenParams := options.New(options.NewMapValuesProvider(map[string]interface{}{}))
	tController := InitController{
		ConfigWriter: func(params *options.ParameterBag) (err error) {
			writtenParams = params
			return nil
		},
		PromptReader: &PromptReaderMock{},
	}

	params := config.FromValues(map[string]string{
		config.ServerURL: srv.URL,
		config.Login:     "login",
		config.Password:  "passwords",
	})
	err = tController.InitConfig(context.Background(), params)

	assert.NoError(t, err)
	if err != nil {
		return
	}
	assert.Equal(t, srv.URL, writtenParams.ReadString(config.ServerURL, ""))
	assert.Equal(t, tokenGiven, writtenParams.ReadString(config.Token, ""))
	assert.True(t, statusRequested)
}

func TestInitError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tController := InitController{
		ConfigWriter: func(params *options.ParameterBag) (err error) {
			return nil
		},
		PromptReader: &PromptReaderMock{},
	}

	params := config.FromValues(map[string]string{
		config.ServerURL: srv.URL,
		config.Login:     "log1123",
		config.Password:  "pass111",
	})
	err := tController.InitConfig(context.Background(), params)

	assert.EqualError(t, err, "config verification failed against the rport: operation failed")
}
