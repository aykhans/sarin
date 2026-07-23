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
	helpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#d1d1d1"))
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FC5B5B")).Bold(true)
	infoStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#5BC0FC")).Bold(true)

	errorLabel = errorStyle.Render("ERROR: ")
	infoLabel  = infoStyle.Render("INFO: ")

	logChannelStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("#757575")).
			PaddingLeft(1).
			Margin(1, 0, 0, 0).
			Foreground(lipgloss.Color("#888888"))
)

// renderRuntimeLog builds a single styled log line for the TUI log box.
func renderRuntimeLog(log runtimeLog) string {
	label := errorLabel
	if log.level == runtimeLogLevelInfo {
		label = infoLabel
	}
	return "[" + log.timestamp.Format("15:04:05") + "] " + label + log.text
}

// renderLogBox renders the styled log box, or "" when there are no lines.
func renderLogBox(logs []string) string {
	var b strings.Builder
	for i, line := range logs {
		if len(line) > 0 {
			b.WriteString(line)
			if i < len(logs)-1 {
				b.WriteString("\n")
			}
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return logChannelStyle.Render(b.String())
}

func helpLine(cancelling bool) string {
	if cancelling {
		return helpStyle.Render("Stopping... (Ctrl+C again to force)")
	}
	return helpStyle.Render("Press Ctrl+C to quit")
}

type progressModel struct {
	progress   progress.Model
	startTime  time.Time
	logs       []string
	counter    *atomic.Uint64
	maxValue   uint64
	showBar    bool
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

	case runtimeLog:
		m.logs = append(m.logs[1:], renderRuntimeLog(msg))
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
	var b strings.Builder
	if box := renderLogBox(m.logs); box != "" {
		b.WriteString(box)
		b.WriteString("\n")
	}

	if m.showBar {
		current := m.counter.Load()
		b.WriteString("\n ")
		b.WriteString(strconv.FormatUint(current, 10))
		b.WriteString("/")
		b.WriteString(strconv.FormatUint(m.maxValue, 10))
		b.WriteString(" - ")
		b.WriteString(time.Since(m.startTime).Round(time.Second / 10).String())
		b.WriteString("\n ")
		b.WriteString(m.progress.ViewAs(float64(current) / float64(m.maxValue)))
	}

	b.WriteString("\n\n  ")
	b.WriteString(helpLine(m.cancelling))
	return b.String()
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
	logs       []string
	showBar    bool
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

	case runtimeLog:
		m.logs = append(m.logs[1:], renderRuntimeLog(msg))
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
	var b strings.Builder
	if box := renderLogBox(m.logs); box != "" {
		b.WriteString(box)
		b.WriteString("\n")
	}

	// Without a spinner, the view is just the log box (plus help until quit).
	if !m.showBar {
		if !m.quit {
			b.WriteString("\n\n  ")
			b.WriteString(helpLine(m.cancelling))
		}
		return b.String()
	}

	if m.quit {
		b.WriteString("\n  ")
		b.WriteString(strconv.FormatUint(m.counter.Load(), 10))
		b.WriteString("  ")
		b.WriteString(infiniteProgressStyle.Render("∙∙∙∙∙"))
		b.WriteString("  ")
		b.WriteString(time.Since(m.startTime).Round(time.Second / 10).String())
		b.WriteString("\n\n")
	} else {
		b.WriteString("\n  ")
		b.WriteString(strconv.FormatUint(m.counter.Load(), 10))
		b.WriteString("  ")
		b.WriteString(m.spinner.View())
		b.WriteString("  ")
		b.WriteString(time.Since(m.startTime).Round(time.Second / 10).String())
		b.WriteString("\n\n  ")
		b.WriteString(helpLine(m.cancelling))
	}
	return b.String()
}

func (s sarin) streamProgress(
	ctx context.Context,
	stopCtrl *StopController,
	done chan<- struct{},
	total uint64,
	counter *atomic.Uint64,
	logChannel <-chan runtimeLog,
	showBar bool,
) {
	var program *tea.Program
	if total > 0 {
		model := progressModel{
			progress:  progress.New(progress.WithGradient("#151594", "#00D4FF")),
			startTime: time.Now(),
			logs:      make([]string, 8),
			counter:   counter,
			maxValue:  total,
			showBar:   showBar,
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
			logs:      make([]string, 8),
			showBar:   showBar,
			ctx:       ctx,
			stop:      stopCtrl.Stop,
			quit:      false,
		}

		program = tea.NewProgram(model)
	}

	stopCtrl.AttachProgram(program)
	defer stopCtrl.AttachProgram(nil)

	go func() {
		for msg := range logChannel {
			program.Send(msg)
		}
	}()

	if _, err := program.Run(); err != nil {
		panic(err)
	}

	done <- struct{}{}
}
