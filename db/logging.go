package db

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"

	"github.com/getsentry/sentry-go"
)

var (
	logNone  *log.Logger
	logDebug *log.Logger
	logInfo  *log.Logger
	logWarn  *log.Logger
	logError *log.Logger
	Trace    *log.Logger
)

func init() {
	IsPrd := os.Getenv(ENV) == "production"

	logNone = log.New(os.Stdout, "", 0)
	logDebug = log.New(os.Stdout, "[Debug] ", 0)
	logInfo = log.New(os.Stdout, " [Info] ", 0)
	logError = log.New(os.Stderr, "[Error] ", 0)
	logWarn = log.New(os.Stdout, " [Warn] ", 0)
	Trace = log.New(os.Stderr, "[TRACE] ", 0)
	Trace.SetFlags(log.Ltime | log.Lshortfile)

	logNone.SetOutput(ioutil.Discard)
	logDebug.SetOutput(ioutil.Discard)

	if !IsPrd {
		logNone.SetOutput(os.Stdout)
		logDebug.SetOutput(os.Stdout)
		logDebug.SetFlags(log.Ltime)
		logInfo.SetFlags(log.Ltime)
		logWarn.SetFlags(log.Ltime)
		logError.SetFlags(log.Ltime | log.Lshortfile)
	}
}

func DisableOutput() {
	logNone.SetOutput(ioutil.Discard)
	logDebug.SetOutput(ioutil.Discard)
	logInfo.SetOutput(ioutil.Discard)
	logWarn.SetOutput(ioutil.Discard)
	logError.SetOutput(ioutil.Discard)
}

func newline() {
	logNone.Print("\n")
}

func Debug(v ...any) {
	logDebug.Println(v...)
}

func Debugf(format string, v ...any) {
	logDebug.Printf(format, v...)
}
func Debugv(v ...any) {
	for i := range v {
		logDebug.Println("Inspect :", reflect.TypeOf(v[i]).String())
		result, _ := json.MarshalIndent(v[i], "", "  ")
		logNone.Print(string(result))
	}
	logNone.Println("")
}

func Info(v ...any) {
	logInfo.Println(v...)
}

func Infof(format string, v ...any) {
	logInfo.Printf(format, v...)
}

func Warn(v ...any) {
	logWarn.Println(v...)
}

func Warnf(format string, v ...any) {
	logWarn.Printf(format, v...)
}

func Error(v ...any) {
	logError.Println(v...)
	sentry.CaptureException(v[0].(error))
}
func Errorf(format string, v ...any) {
	logError.Printf(format, v...)
	sentry.CaptureException(fmt.Errorf(format, v...))
}
