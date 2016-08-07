package loggingtest

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

type logSubscription struct {
	exp      string
	n        int
	response chan<- struct{}
}

type logWatch struct {
	entries []string
	reqs    []*logSubscription
}

// Logger provides an implementation of the logging.Logger interface
// that can be used to receive notifications about log events.
type Logger struct {
	save   chan string
	notify chan<- logSubscription
	clear  chan struct{}
	quit   chan<- struct{}
}

// ErrWaitTimeout is returned when a logging event doesn't happen
// within a timeout.
var ErrWaitTimeout = errors.New("timeout")

func (lw *logWatch) save(e string) {
	lw.entries = append(lw.entries, e)
	for i := len(lw.reqs) - 1; i >= 0; i-- {
		req := lw.reqs[i]
		if strings.Contains(e, req.exp) {
			req.n--
			if req.n <= 0 {
				close(req.response)
				lw.reqs = append(lw.reqs[:i], lw.reqs[i+1:]...)
			}
		}
	}
}

func (lw *logWatch) notify(req logSubscription) {
	for i := len(lw.entries) - 1; i >= 0; i-- {
		if strings.Contains(lw.entries[i], req.exp) {
			req.n--
			if req.n == 0 {
				break
			}
		}
	}

	if req.n <= 0 {
		close(req.response)
	} else {
		lw.reqs = append(lw.reqs, &req)
	}
}

func (lw *logWatch) clear() {
	lw.entries = nil
	lw.reqs = nil
}

// Returns a new, initialized instance of Logger.
func New() *Logger {
	lw := &logWatch{}
	save := make(chan string)
	notify := make(chan logSubscription)
	clear := make(chan struct{})
	quit := make(chan struct{})

	go func() {
		for {
			select {
			case e := <-save:
				lw.save(e)
			case req := <-notify:
				lw.notify(req)
			case <-clear:
				lw.clear()
			case <-quit:
				return
			}
		}
	}()

	return &Logger{save, notify, clear, quit}
}

func (tl *Logger) logf(f string, a ...interface{}) {
	log.Printf(f, a...)
	tl.save <- fmt.Sprintf(f, a...)
}

func (tl *Logger) log(a ...interface{}) {
	log.Println(a...)
	tl.save <- fmt.Sprint(a...)
}

// Returns nil when n logging events matching exp were received or returns
// ErrWaitTimeout when to timeout expired.
func (tl *Logger) WaitForN(exp string, n int, to time.Duration) error {
	found := make(chan struct{}, 1)
	tl.notify <- logSubscription{exp, n, found}

	select {
	case <-found:
		return nil
	case <-time.After(to):
		return ErrWaitTimeout
	}
}

// Returns nil when a logging event matching exp was received or returns
// ErrWaitTimeout when to timeout expired.
func (tl *Logger) WaitFor(exp string, to time.Duration) error {
	return tl.WaitForN(exp, 1, to)
}

// Clears the stored logging events.
func (tl *Logger) Reset() {
	tl.clear <- struct{}{}
}

// Closes the logger.
func (tl *Logger) Close() {
	close(tl.quit)
}

func (tl *Logger) Error(a ...interface{})            { tl.log(a...) }
func (tl *Logger) Errorf(f string, a ...interface{}) { tl.logf(f, a...) }
func (tl *Logger) Warn(a ...interface{})             { tl.log(a...) }
func (tl *Logger) Warnf(f string, a ...interface{})  { tl.logf(f, a...) }
func (tl *Logger) Info(a ...interface{})             { tl.log(a...) }
func (tl *Logger) Infof(f string, a ...interface{})  { tl.logf(f, a...) }
func (tl *Logger) Debug(a ...interface{})            { tl.log(a...) }
func (tl *Logger) Debugf(f string, a ...interface{}) { tl.logf(f, a...) }
