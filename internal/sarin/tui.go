package sarin

import (
	"context"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg time.Time

var (
	helpStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("#d1d1d1"))
	errorStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#FC5B5B")).Bold(true)
	warningStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD93D")).Bold(true)
	messageChannelStyle = lipgloss.NewStyle().
				Border(lipgloss.ThickBorder(), false, false, false, true).
				BorderForeground(lipgloss.Color("#757575")).
				PaddingLeft(1).
				Margin(1, 0, 0, 0).
				Foreground(lipgloss.Color("#888888"))
)

type progressModel struct {
	progress   progress.Model
	startTime  time.Time
	messages   []string
	counter    *atomic.Uint64
	current    uint64
	maxValue   uint64
	ctx        context.Context //nolint:containedctx
	stop       func()
	cancelling bool
}

func (m progressModel) Init() tea.Cmd {
	return tea.Batch(progressTickCmd())
}

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.cancelling = true
			m.stop()
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.progress.Width = max(10, msg.Width-1)
		if m.ctx.Err() != nil {
			return m, tea.Quit
		}
		return m, nil

	case runtimeMessage:
		var msgBuilder strings.Builder
		msgBuilder.WriteString("[")
		msgBuilder.WriteString(msg.timestamp.Format("15:04:05"))
		msgBuilder.WriteString("] ")
		switch msg.level {
		case runtimeMessageLevelError:
			msgBuilder.WriteString(errorStyle.Render("ERROR: "))
		case runtimeMessageLevelWarning:
			msgBuilder.WriteString(warningStyle.Render("WARNING: "))
		}
		msgBuilder.WriteString(msg.text)
		m.messages = append(m.messages[1:], msgBuilder.String())
		if m.ctx.Err() != nil {
			return m, tea.Quit
		}
		return m, nil

	case tickMsg:
		if m.ctx.Err() != nil {
			return m, tea.Quit
		}
		return m, progressTickCmd()

	default:
		if m.ctx.Err() != nil {
			return m, tea.Quit
		}
		return m, nil
	}
}

func (m progressModel) View() string {
	var messagesBuilder strings.Builder
	for i, msg := range m.messages {
		if len(msg) > 0 {
			messagesBuilder.WriteString(msg)
			if i < len(m.messages)-1 {
				messagesBuilder.WriteString("\n")
			}
		}
	}

	var finalBuilder strings.Builder
	if messagesBuilder.Len() > 0 {
		finalBuilder.WriteString(messageChannelStyle.Render(messagesBuilder.String()))
		finalBuilder.WriteString("\n")
	}

	m.current = m.counter.Load()
	finalBuilder.WriteString("\n ")
	finalBuilder.WriteString(strconv.FormatUint(m.current, 10))
	finalBuilder.WriteString("/")
	finalBuilder.WriteString(strconv.FormatUint(m.maxValue, 10))
	finalBuilder.WriteString(" - ")
	finalBuilder.WriteString(time.Since(m.startTime).Round(time.Second / 10).String())
	finalBuilder.WriteString("\n ")
	finalBuilder.WriteString(m.progress.ViewAs(float64(m.current) / float64(m.maxValue)))
	finalBuilder.WriteString("\n\n  ")
	if m.cancelling {
		finalBuilder.WriteString(helpStyle.Render("Stopping... (Ctrl+C again to force)"))
	} else {
		finalBuilder.WriteString(helpStyle.Render("Press Ctrl+C to quit"))
	}
	return finalBuilder.String()
}

func progressTickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*250, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

var infiniteProgressStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D4FF"))

type infiniteProgressModel struct {
	spinner    spinner.Model
	startTime  time.Time
	counter    *atomic.Uint64
	messages   []string
	ctx        context.Context //nolint:containedctx
	quit       bool
	stop       func()
	cancelling bool
}

func (m infiniteProgressModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m infiniteProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.cancelling = true
			m.stop()
		}
		return m, nil

	case runtimeMessage:
		var msgBuilder strings.Builder
		msgBuilder.WriteString("[")
		msgBuilder.WriteString(msg.timestamp.Format("15:04:05"))
		msgBuilder.WriteString("] ")
		switch msg.level {
		case runtimeMessageLevelError:
			msgBuilder.WriteString(errorStyle.Render("ERROR: "))
		case runtimeMessageLevelWarning:
			msgBuilder.WriteString(warningStyle.Render("WARNING: "))
		}
		msgBuilder.WriteString(msg.text)
		m.messages = append(m.messages[1:], msgBuilder.String())
		if m.ctx.Err() != nil {
			m.quit = true
			return m, tea.Quit
		}
		return m, nil

	default:
		if m.ctx.Err() != nil {
			m.quit = true
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m infiniteProgressModel) View() string {
	var messagesBuilder strings.Builder
	for i, msg := range m.messages {
		if len(msg) > 0 {
			messagesBuilder.WriteString(msg)
			if i < len(m.messages)-1 {
				messagesBuilder.WriteString("\n")
			}
		}
	}

	var finalBuilder strings.Builder
	if messagesBuilder.Len() > 0 {
		finalBuilder.WriteString(messageChannelStyle.Render(messagesBuilder.String()))
		finalBuilder.WriteString("\n")
	}

	if m.quit {
		finalBuilder.WriteString("\n  ")
		finalBuilder.WriteString(strconv.FormatUint(m.counter.Load(), 10))
		finalBuilder.WriteString("  ")
		finalBuilder.WriteString(infiniteProgressStyle.Render("∙∙∙∙∙"))
		finalBuilder.WriteString("  ")
		finalBuilder.WriteString(time.Since(m.startTime).Round(time.Second / 10).String())
		finalBuilder.WriteString("\n\n")
	} else {
		finalBuilder.WriteString("\n  ")
		finalBuilder.WriteString(strconv.FormatUint(m.counter.Load(), 10))
		finalBuilder.WriteString("  ")
		finalBuilder.WriteString(m.spinner.View())
		finalBuilder.WriteString("  ")
		finalBuilder.WriteString(time.Since(m.startTime).Round(time.Second / 10).String())
		finalBuilder.WriteString("\n\n  ")
		if m.cancelling {
			finalBuilder.WriteString(helpStyle.Render("Stopping... (Ctrl+C again to force)"))
		} else {
			finalBuilder.WriteString(helpStyle.Render("Press Ctrl+C to quit"))
		}
	}
	return finalBuilder.String()
}

func (q sarin) streamProgress(
	ctx context.Context,
	stopCtrl *StopController,
	done chan<- struct{},
	total uint64,
	counter *atomic.Uint64,
	messageChannel <-chan runtimeMessage,
) {
	var program *tea.Program
	if total > 0 {
		model := progressModel{
			progress:  progress.New(progress.WithGradient("#151594", "#00D4FF")),
			startTime: time.Now(),
			messages:  make([]string, 8),
			counter:   counter,
			current:   0,
			maxValue:  total,
			ctx:       ctx,
			stop:      stopCtrl.Stop,
		}

		program = tea.NewProgram(model)
	} else {
		model := infiniteProgressModel{
			spinner: spinner.New(
				spinner.WithSpinner(
					spinner.Spinner{
						Frames: []string{
							"●∙∙∙∙",
							"∙●∙∙∙",
							"∙∙●∙∙",
							"∙∙∙●∙",
							"∙∙∙∙●",
							"∙∙∙●∙",
							"∙∙●∙∙",
							"∙●∙∙∙",
						},
						FPS: time.Second / 8, //nolint:mnd
					},
				),
				spinner.WithStyle(infiniteProgressStyle),
			),
			startTime: time.Now(),
			counter:   counter,
			messages:  make([]string, 8),
			ctx:       ctx,
			stop:      stopCtrl.Stop,
			quit:      false,
		}

		program = tea.NewProgram(model)
	}

	stopCtrl.AttachProgram(program)
	defer stopCtrl.AttachProgram(nil)

	go func() {
		for msg := range messageChannel {
			program.Send(msg)
		}
	}()

	if _, err := program.Run(); err != nil {
		panic(err)
	}

	done <- struct{}{}
}
