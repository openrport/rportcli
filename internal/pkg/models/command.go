package models

import (
	"strconv"
	"time"

	"github.com/breathbath/go_utils/v2/pkg/testing"
)

type JobResult struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

type Job struct {
	Jid         string    `json:"jid" yaml:"jid"`
	Status      string    `json:"status" yaml:"status"`
	FinishedAt  time.Time `json:"finished_at" yaml:"finished_at"`
	ClientID    string    `json:"client_id" yaml:"client_id"`
	ClientName  string    `json:"client_name,omitempty" yaml:"client_name"`
	Command     string    `json:"command" yaml:"command"`
	Cwd         string    `json:"cwd" yaml:"cwd"`
	Pid         int       `json:"pid" yaml:"pid"`
	StartedAt   time.Time `json:"started_at" yaml:"started_at"`
	CreatedBy   string    `json:"created_by" yaml:"created_by"`
	MultiJobID  string    `json:"multi_job_id" yaml:"multi_job_id"`
	TimeoutSec  int       `json:"timeout_sec" yaml:"timeout_sec"`
	Error       string    `json:"error" yaml:"error"`
	Result      JobResult `json:"result" yaml:"result"`
	IsSudo      bool      `json:"is_sudo" yaml:"is_sudo"`
	IsScript    bool      `json:"is_script" yaml:"is_script"`
	Interpreter string    `json:"interpreter" yaml:"interpreter"`
}

type WsScriptCommand struct {
	ClientIDs           []string `json:"client_ids"`
	GroupIDs            []string `json:"group_ids,omitempty"`
	IsSudo              bool     `json:"is_sudo"`
	ExecuteConcurrently bool     `json:"execute_concurrently"`
	AbortOnError        bool     `json:"abort_on_error"`
	TimeoutSec          int      `json:"timeout_sec"`
	Command             string   `json:"command"`
	Script              string   `json:"script"`
	Cwd                 string   `json:"cwd"`
	Interpreter         string   `json:"interpreter"`
}

func (j *Job) KeyValues() []testing.KeyValueStr {
	return []testing.KeyValueStr{
		{
			Key:   "Job ID",
			Value: j.Jid,
		},
		{
			Key:   "Status",
			Value: j.Status,
		},
		{
			Key:   "Command Output",
			Value: j.Result.Stdout,
		},
		{
			Key:   "Command Error Output",
			Value: j.Result.Stderr,
		},
		{
			Key:   "Started at",
			Value: j.StartedAt.Format(time.RFC3339),
		},
		{
			Key:   "Finished at",
			Value: j.FinishedAt.Format(time.RFC3339),
		},
		{
			Key:   "Client ID",
			Value: j.ClientID,
		},
		{
			Key:   "Command",
			Value: j.Command,
		},
		{
			Key:   "Pid",
			Value: strconv.Itoa(j.Pid),
		},
		{
			Key:   "Timeout sec",
			Value: strconv.Itoa(j.TimeoutSec),
		},
		{
			Key:   "Created By",
			Value: j.CreatedBy,
		},
		{
			Key:   "Multi Job ID",
			Value: j.MultiJobID,
		},
		{
			Key:   "Interpreter",
			Value: j.Interpreter,
		},
	}
}
