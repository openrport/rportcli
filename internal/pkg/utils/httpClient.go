package utils

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/breathbath/go_utils/v2/pkg/env"
	"github.com/sirupsen/logrus"
)

const (
	maxValidResponseCode = 399
	connectionTimeoutSec = 10
)

type BaseClient struct {
	auth Auth
}

func (c *BaseClient) WithAuth(a Auth) {
	c.auth = a
}

func (c *BaseClient) buildClient() *http.Client {
	connectionTimeout := env.ReadEnvInt("CONN_TIMEOUT_SEC", connectionTimeoutSec)
	transport := &http.Transport{
		DisableKeepAlives:     true,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: false},
		ResponseHeaderTimeout: time.Duration(connectionTimeout) * time.Second,
	}
	cl := &http.Client{Transport: transport}

	return cl
}

func (c *BaseClient) Call(req *http.Request, target interface{}, errTarget error) (resp *http.Response, err error) {
	cl := c.buildClient()
	dump, _ := httputil.DumpRequest(req, true)
	if urlString, err := url.QueryUnescape(string(dump)); err == nil {
		logrus.Debugf("API request (url decoded): %s", urlString)
	} else {
		logrus.Debugf("API request (raw): %s", string(dump))
	}

	if c.auth != nil {
		err = c.auth.AuthRequest(req)
		if err != nil {
			return nil, err
		}
	}

	resp, err = cl.Do(req)
	if err != nil {
		return resp, fmt.Errorf("operation failed with an error: %v", err)
	}
	var respBodyBytes []byte
	if resp.StatusCode > maxValidResponseCode {
		respBodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			logrus.Warnf("failed to read response body: %v", err)
			e := c.convertResponseCodeToError(resp.StatusCode)
			return resp, e
		}

		err = json.Unmarshal(respBodyBytes, errTarget)
		if err != nil {
			logrus.Warnf("cannot unmarshal error response %s: %v", string(respBodyBytes), err)
			e := c.convertResponseCodeToError(resp.StatusCode)
			return resp, e
		}
		return resp, errTarget
	}

	if target == nil {
		return resp, nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err == io.EOF {
		return resp, errors.New("no data received from command execution")
	}
	if err != nil {
		return resp, fmt.Errorf("can't parse data from command execution: %v", err)
	}

	logrus.Debugf("Got response:\n'%s'\nstatus code: '%d'", PrettifyJSON(respBody), resp.StatusCode)

	err = json.Unmarshal(respBody, target)
	if err != nil {
		return resp, fmt.Errorf("can't parse data from command execution: %v", err)
	}

	return resp, nil
}

func (c *BaseClient) convertResponseCodeToError(respCode int) (err error) {
	if respCode == http.StatusNotFound {
		err = errors.New("the specified item doesn't exist")
	} else if respCode == http.StatusInternalServerError {
		err = errors.New("operation failed")
	} else if respCode == http.StatusBadRequest {
		err = errors.New("invalid input provided")
	} else {
		err = fmt.Errorf("unknown error: %d %s", respCode, http.StatusText(respCode))
	}

	return err
}
