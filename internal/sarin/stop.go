package sarin

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
)

const forceExitCode = 130

// StopController coordinates a two-stage shutdown.
//
// The first Stop call cancels the supplied context so workers and the job
// loop can drain. The second Stop call restores the terminal (if a bubbletea
// program has been attached) and calls os.Exit(forceExitCode), bypassing any
// in-flight captcha polls, Lua/JS scripts, or HTTP requests that would
// otherwise keep the process alive.
type StopController struct {
	count   atomic.Int32
	cancel  func()
	mu      sync.Mutex
	program *tea.Program
}

func NewStopController(cancel func()) *StopController {
	return &StopController{cancel: cancel}
}

// AttachProgram registers the active bubbletea program so the terminal state
// can be restored before os.Exit on the forced shutdown path. Pass nil to
// detach once the program has finished.
func (s *StopController) AttachProgram(program *tea.Program) {
	s.mu.Lock()
	s.program = program
	s.mu.Unlock()
}

func (s *StopController) Stop() {
	switch s.count.Add(1) {
	case 1:
		s.cancel()
	case 2:
		s.mu.Lock()
		p := s.program
		s.mu.Unlock()
		if p != nil {
			_ = p.ReleaseTerminal()
		}
		fmt.Fprintln(os.Stderr, "killing...")
		os.Exit(forceExitCode)
	}
}
