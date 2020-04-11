/*
 *  b4ck-client
 *  Copyright 2020 Michał Trojnara

 *  This program is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.

 *  This program is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.

 *  You should have received a copy of the GNU General Public License
 *  along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/fatih/color"
)

const (
	UNSPECIFIED = iota
	ERROR
	WARNING
	INFO
	DEBUG
)

type Level int

func (level Level) String() string {
	switch level {
	case UNSPECIFIED:
		return color.HiCyanString("UNSPECIFIED")
	case ERROR:
		return color.HiRedString("ERROR")
	case WARNING:
		return color.YellowString("WARNING")
	case INFO:
		return color.HiBlueString("INFO")
	case DEBUG:
		return color.HiGreenString("DEBUG")
	default:
		return color.HiMagentaString("INVALID")
	}
}

// Currently unused
func (level Level) Color() *color.Color {
	switch level {
	case UNSPECIFIED:
		return color.New(color.FgCyan)
	case ERROR:
		return color.New(color.FgRed)
	case WARNING:
		return color.New(color.FgYellow)
	case INFO:
		return color.New(color.FgBlue)
	case DEBUG:
		return color.New(color.FgGreen)
	default:
		return color.New(color.FgCyan)
	}
}

func ParseLevel(level string) (Level, bool) {
	switch strings.ToUpper(level) {
	case "ERROR":
		return ERROR, true
	case "WARNING":
		return WARNING, true
	case "INFO":
		return INFO, true
	case "DEBUG":
		return DEBUG, true
	default:
		return UNSPECIFIED, false
	}
}

type Logger struct {
	name   string
	level  Level
	logger *log.Logger
}

func GetLogger(name string) *Logger {
	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)
	logger.SetOutput(color.Output)
	return &Logger{
		name:   name,
		level:  UNSPECIFIED,
		logger: logger,
	}
}

func (l *Logger) Child(name string) *Logger {
	logger := *l
	logger.name = fmt.Sprintf("%s.%s", l.name, name)
	return &logger
}

func (l *Logger) SetLogLevel(level Level) {
	l.level = level
}

func (l *Logger) printf(level Level, format string, args ...interface{}) {
	if level > l.level {
		return
	}

	ourFormat := ""
	ourArgs := make([]interface{}, 0)

	if l.level >= DEBUG { // Performance and readability optimization
		_, file, line, ok := runtime.Caller(2)
		if ok {
			ourFormat += "%s:%d "
			ourArgs = append(ourArgs, path.Base(file), line)
		}
	}

	ourFormat += "%s %s: "
	ourArgs = append(ourArgs, l.name, level)

	l.logger.Printf(ourFormat+format, append(ourArgs, args...)...)
	// level.Color().Printf(ourFormat+format+"\n", append(ourArgs, args...)...)
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	l.printf(ERROR, format, args...)
}

func (l *Logger) Warningf(format string, args ...interface{}) {
	l.printf(WARNING, format, args...)
}

func (l *Logger) Infof(format string, args ...interface{}) {
	l.printf(INFO, format, args...)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.printf(DEBUG, format, args...)
}

// vim: noet:ts=4:sw=4:sts=4:spell
