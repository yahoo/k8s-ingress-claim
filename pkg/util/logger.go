// Copyright 2017 Yahoo Holdings Inc.
// Licensed under the terms of the 3-Clause BSD License.
package util

import (
	"bytes"
	"github.com/Sirupsen/logrus"
	"io"
	"os"
	"strings"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"
)

type Formatter struct {
}

var (
	logger *logrus.Logger
	once   sync.Once
)

func (f *Formatter) Format(entry *logrus.Entry) ([]byte, error) {
	b := &bytes.Buffer{}
	s := strings.ToUpper(entry.Level.String()) + " [" + entry.Time.Format("2006-01-02 15:04:05") + "] " + entry.Message

	b.WriteString(s)
	b.WriteByte('\n')
	return b.Bytes(), nil
}

func createLogger(writer io.Writer, level string) *logrus.Logger {
	logLevel, _ := logrus.ParseLevel(level)

	myLogger := &logrus.Logger{
		Out:       writer,
		Formatter: new(Formatter),
		Level:     logLevel,
	}
	return myLogger

}

func GetLogger(logFilename string, level string) *logrus.Logger {
	once.Do(func() {
		fileWriter := io.MultiWriter(os.Stdout, &lumberjack.Logger{
			Filename:   logFilename,
			MaxSize:    1, // Mb
			MaxBackups: 5,
			MaxAge:     28, // Days
		})

		logger = createLogger(fileWriter, level)
	})
	return logger
}
