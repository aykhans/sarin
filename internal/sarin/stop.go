package sarin

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
)

const (
	forceExitCode = 130
	// forceExitWait bounds the wait for the program to restore the terminal.
	forceExitWait = 2 * time.Second
	// killingWait bounds the "killing..." write, which a stuck terminal blocks.
	killingWait = 100 * time.Millisecond
)

// StopController is the single place a run is stopped from.
// The first Stop drains the run, the second exits at once.
type StopController struct {
	count       atomic.Int32
	cancel      func()
	mu          sync.Mutex
	program     *tea.Program
	programDone chan struct{} // closed when Run returns
	ttyState    *term.State   // stdin before the program made it raw
}

func NewStopController(cancel func()) *StopController {
	return &StopController{cancel: cancel}
}

// RunProgram runs p so a forced exit can quit it and wait for it.
func (s *StopController) RunProgram(p *tea.Program) (tea.Model, error) {
	var ttyState *term.State
	if term.IsTerminal(os.Stdin.Fd()) {
		ttyState, _ = term.GetState(os.Stdin.Fd())
	}

	done := make(chan struct{})
	s.mu.Lock()
	s.program, s.programDone, s.ttyState = p, done, ttyState
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.program, s.programDone, s.ttyState = nil, nil, nil
		s.mu.Unlock()
		close(done)
	}()

	return p.Run()
}

// ListenSignals routes SIGINT to Stop. SIGTERM comes from programs like timeout,
// which may send it twice, so it only ever stops gracefully.
func (s *StopController) ListenSignals() {
	sigChan := make(chan os.Signal, 4)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	for sig := range sigChan {
		if sig == syscall.SIGTERM {
			s.stopGracefully()
			continue
		}
		s.Stop()
	}
}

// Stopping reports whether a stop has been requested.
func (s *StopController) Stopping() bool {
	return s.count.Load() > 0
}

func (s *StopController) Stop() {
	switch s.count.Add(1) {
	case 1:
		s.cancel()
	case 2:
		go s.forceExit()
	}
}

// stopGracefully starts the graceful stop unless a stop has already started.
func (s *StopController) stopGracefully() {
	if s.count.CompareAndSwap(0, 1) {
		s.cancel()
	}
}

// forceExit quits the program, waits up to forceExitWait for it, then exits.
func (s *StopController) forceExit() {
	s.mu.Lock()
	p, programDone, ttyState := s.program, s.programDone, s.ttyState
	s.mu.Unlock()

	quit := make(chan struct{})
	go func() {
		if p != nil {
			p.Quit()
			<-programDone
		}
		close(quit)
	}()

	select {
	case <-quit:
	case <-time.After(forceExitWait):
		if ttyState != nil {
			_ = term.Restore(os.Stdin.Fd(), ttyState)
		}
	}

	printed := make(chan struct{})
	go func() {
		fmt.Fprintln(os.Stderr, "killing...")
		close(printed)
	}()
	select {
	case <-printed:
	case <-time.After(killingWait):
	}
	os.Exit(forceExitCode)
}
