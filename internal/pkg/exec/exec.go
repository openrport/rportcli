package exec

import (
	"io"
	"os/exec"

	"github.com/sirupsen/logrus"
)

type Executor struct {
	CommandProvider func(filePath string) (cmd string, args []string)
	StdOut          io.Writer
	Stdin           io.Reader
	StdErr          io.Writer
}

func (re *Executor) StartDefaultApp(filePath string) error {
	rdpCmd, args := re.CommandProvider(filePath)
	c := exec.Command(rdpCmd, args...)

	c.Stdout = re.StdOut
	c.Stdin = re.Stdin
	c.Stderr = re.StdErr

	err := c.Run()
	logrus.Debugf("will run %s", c.String())
	if err != nil {
		return err
	}
	logrus.Debugf("finished run %s", c.String())

	return nil
}
