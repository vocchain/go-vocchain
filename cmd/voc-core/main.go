package main

import (
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tendermint/tmlibs/cli"

	"github.com/vocchain/go-vocchain/cmd/voc-core/commands"
	"github.com/vocchain/go-vocchain/config"
)

// ContextHook is a hook for logrus.
type ContextHook struct{}

// Levels returns the whole levels.
func (hook ContextHook) Levels() []log.Level {
	return log.AllLevels
}

// Fire helps logrus record the related file, function and line.
func (hook ContextHook) Fire(entry *log.Entry) error {
	pc := make([]uintptr, 3, 3)
	cnt := runtime.Callers(6, pc)

	for i := 0; i < cnt; i++ {
		fu := runtime.FuncForPC(pc[i] - 1)
		name := fu.Name()
		if !strings.Contains(name, "github.com/Sirupsen/log") {
			file, line := fu.FileLine(pc[i] - 1)
			entry.Data["file"] = path.Base(file)
			entry.Data["func"] = path.Base(name)
			entry.Data["line"] = line
			break
		}
	}
	return nil
}

func init() {
	log.SetFormatter(&log.TextFormatter{TimestampFormat: time.StampMilli, DisableColors: true})

	// If environment variable VOCCHAIN_DEBUG is not empty,
	// then add the hook to logrus and set the log level to DEBUG
	if os.Getenv("VOCCHAIN_DEBUG") != "" {
		log.AddHook(ContextHook{})
	}
}

func main() {
	cmd := cli.PrepareBaseCmd(commands.RootCmd, "VOC", os.ExpandEnv(config.DefaultDataDir()))
	cmd.Execute()
}
