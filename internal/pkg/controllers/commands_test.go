package controllers

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/cloudradar-monitoring/rportcli/internal/pkg/config"

	"github.com/cloudradar-monitoring/rportcli/internal/pkg/models"
	"github.com/stretchr/testify/assert"
)

type ReadChunk struct {
	Output []byte
	Err    error
}

type ReadWriterMock struct {
	itemsToRead   []ReadChunk
	itemReadIndex int
	writtenItems  []string
	writeError    error
	isClosed      bool
	closeError    error
}

func (rwm *ReadWriterMock) Read() (msg []byte, err error) {
	item := rwm.itemsToRead[rwm.itemReadIndex]

	msg = item.Output
	err = item.Err

	rwm.itemReadIndex++

	return
}

func (rwm *ReadWriterMock) Write(inputMsg []byte) (n int, err error) {
	rwm.writtenItems = append(rwm.writtenItems, string(inputMsg))
	return 0, rwm.writeError
}

func (rwm *ReadWriterMock) Close() error {
	rwm.isClosed = true
	return rwm.closeError
}

type JobRendererMock struct {
	jobToRender *models.Job
	err         error
}

func (jrm *JobRendererMock) RenderJob(j *models.Job) error {
	jrm.jobToRender = j
	return jrm.err
}

func TestInteractiveCommandExecutionSuccess(t *testing.T) {
	jobResp := models.Job{
		Jid:        "123",
		Status:     "done",
		FinishedAt: time.Now(),
		ClientID:   "123",
		Command:    "ls",
		Shell:      "sh",
		Pid:        12,
		StartedAt:  time.Now(),
		CreatedBy:  "admin",
		TimeoutSec: 1,
		Result: models.JobResult{
			Stdout: "some out",
			Stderr: "some err",
		},
	}
	jobRespBytes, err := json.Marshal(jobResp)
	assert.NoError(t, err)
	if err != nil {
		return
	}

	rw := &ReadWriterMock{
		itemsToRead: []ReadChunk{
			{
				Output: jobRespBytes,
			},
			{
				Err: io.EOF,
			},
		},
		writtenItems: []string{},
		isClosed:     false,
	}

	pr := &PromptReaderMock{
		ReadOutputs:         []string{},
		PasswordReadOutputs: []string{},
	}

	jr := &JobRendererMock{}

	ic := &InteractiveCommandsController{
		ReadWriter:   rw,
		JobRenderer:  jr,
	}

	params := config.FromValues(map[string]string{
		ClientIDs:        "1235",
		Command:          "cmd",
		Timeout:          "1",
		GroupIDs:         "333",
		ExecConcurrently: "1",
	})
	err = ic.Start(context.Background(), params)

	assert.NoError(t, err)

	assert.Equal(t, pr.PasswordReadCount, 0)
	assert.Equal(t, pr.ReadCount, 0)

	assert.Len(t, rw.writtenItems, 1)
	expectedCommandInput := `{"command":"cmd","client_ids":["1235"],"group_ids":["333"],"timeout_sec":1,"execute_concurrently":true}`
	assert.Equal(t, expectedCommandInput, rw.writtenItems[0])

	assert.NotNil(t, jr.jobToRender)
	actualJobRenderResult, err := json.Marshal(jr.jobToRender)
	assert.NoError(t, err)
	assert.Equal(t, string(jobRespBytes), string(actualJobRenderResult))
	assert.True(t, rw.isClosed)
}

func TestInteractiveCommandExecutionWithInvalidResponse(t *testing.T) {
	resp := models.ErrorResp{
		Errors: []models.Error{
			{
				Code:   "500",
				Title:  "some error",
				Detail: "some error detail",
			},
		},
	}
	jobRespBytes, err := json.Marshal(resp)
	assert.NoError(t, err)
	if err != nil {
		return
	}
	rw := &ReadWriterMock{
		itemsToRead: []ReadChunk{
			{
				Output: jobRespBytes,
			},
			{
				Err: io.EOF,
			},
		},
		writtenItems: []string{},
		isClosed:     false,
	}

	jr := &JobRendererMock{}

	ic := &InteractiveCommandsController{
		ReadWriter:   rw,
		JobRenderer:  jr,
	}

	params := config.FromValues(map[string]string{
		ClientIDs: "123",
		Command:   "ls",
	})
	err = ic.Start(context.Background(), params)

	assert.Error(t, err)
	if err == nil {
		return
	}
	assert.Contains(t, err.Error(), "some error, code: 500, details: some error detail")
}
