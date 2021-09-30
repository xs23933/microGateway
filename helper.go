package microgateway

import (
	"fmt"
	"io/ioutil"
	"log"
	weakrand "math/rand"
	"net/http"
	"path"
	"runtime"
	"strings"

	"gopkg.in/yaml.v2"
)

func ErrorFunc(w http.ResponseWriter, r *http.Request, status int) {
	WriteTextResp(w, status, fmt.Sprintf("%d %s\n", status, http.StatusText(status)))
}

func WriteTextResp(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(body)); err != nil {
		log.Println("[E] failed to write body: ", err)
	}
}

// LoadConfigFile load yml config
func LoadConfigFile(confFile string) map[string]interface{} {
	conf := make(map[string]interface{})
	buf, err := ioutil.ReadFile(confFile)
	if err != nil {
		conf["debug"] = true
		conf["listen"] = 8080
		conf["ReadTimeout"] = 10
		conf["ReadHeaderTimeout"] = 2
		conf["WriteTimeout"] = 10
		conf["IdleTimeout"] = 4
		conf["MaxHeaderBytes"] = 20480
		yml, _ := yaml.Marshal(conf)
		ioutil.WriteFile(confFile, yml, 0644)
	} else if err = yaml.Unmarshal(buf, &conf); err != nil {
		log.Printf("Config unmarshal err: %v", err)
	}
	configFile = confFile
	return conf
}

func SaveConfigFile(conf map[string]interface{}) error {
	yml, err := yaml.Marshal(conf)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(configFile, yml, 0644)
}

// Error is a convenient way for a Handler to populate the
// essential fields of a HandlerError. If err is itself a
// HandlerError, then any essential fields that are not
// set will be populated.
func Error(statusCode int, err error) HandlerError {
	const idLen = 9
	if he, ok := err.(HandlerError); ok {
		if he.ID == "" {
			he.ID = randString(idLen, true)
		}
		if he.Trace == "" {
			he.Trace = trace()
		}
		if he.StatusCode == 0 {
			he.StatusCode = statusCode
		}
		return he
	}
	return HandlerError{
		ID:         randString(idLen, true),
		StatusCode: statusCode,
		Err:        err,
		Trace:      trace(),
	}
}

// randString returns a string of n random characters.
// It is not even remotely secure OR a proper distribution.
// But it's good enough for some things. It excludes certain
// confusing characters like I, l, 1, 0, O, etc. If sameCase
// is true, then uppercase letters are excluded.
func randString(n int, sameCase bool) string {
	if n <= 0 {
		return ""
	}
	dict := []byte("abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRTUVWXY23456789")
	if sameCase {
		dict = []byte("abcdefghijkmnpqrstuvwxyz0123456789")
	}
	b := make([]byte, n)
	for i := range b {
		//nolint:gosec
		b[i] = dict[weakrand.Int63()%int64(len(dict))]
	}
	return string(b)
}

func trace() string {
	if pc, file, line, ok := runtime.Caller(2); ok {
		filename := path.Base(file)
		pkgAndFuncName := path.Base(runtime.FuncForPC(pc).Name())
		return fmt.Sprintf("%s (%s:%d)", pkgAndFuncName, filename, line)
	}
	return ""
}

type CtxKey string

// ErrorCtxKey is the context key to use when storing
// an error (for use with context.Context).
const ErrorCtxKey = CtxKey("handler_chain_error")

// HandlerError is a serializable representation of
// an error from within an HTTP handler.
type HandlerError struct {
	Err        error // the original error value and message
	StatusCode int   // the HTTP status code to associate with this error

	ID    string // generated; for identifying this error in logs
	Trace string // produced from call stack
}

func (e HandlerError) Error() string {
	var s string
	if e.ID != "" {
		s += fmt.Sprintf("{id=%s}", e.ID)
	}
	if e.Trace != "" {
		s += " " + e.Trace
	}
	if e.StatusCode != 0 {
		s += fmt.Sprintf(": HTTP %d", e.StatusCode)
	}
	if e.Err != nil {
		s += ": " + e.Err.Error()
	}
	return strings.TrimSpace(s)
}

var (
	configFile = "config.yml"
)
