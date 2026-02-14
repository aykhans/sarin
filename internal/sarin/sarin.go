package sarin

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/valyala/fasthttp"
	"go.aykhans.me/sarin/internal/script"
	"go.aykhans.me/sarin/internal/types"
)

type runtimeMessageLevel uint8

const (
	runtimeMessageLevelWarning runtimeMessageLevel = iota
	runtimeMessageLevelError
)

type runtimeMessage struct {
	timestamp time.Time
	level     runtimeMessageLevel
	text      string
}

type messageSender func(level runtimeMessageLevel, text string)

type sarin struct {
	workers        uint
	requestURL     *url.URL
	methods        []string
	params         types.Params
	headers        types.Headers
	cookies        types.Cookies
	bodies         []string
	totalRequests  *uint64
	totalDuration  *time.Duration
	timeout        time.Duration
	quiet          bool
	skipCertVerify bool
	values         []string
	collectStats   bool
	dryRun         bool

	hostClients []*fasthttp.HostClient
	responses   *SarinResponseData
	fileCache   *FileCache
	scriptChain *script.Chain
}

// NewSarin creates a new sarin instance for load testing.
// It can return the following errors:
//   - types.ProxyDialError
//   - types.ErrScriptEmpty
//   - types.ScriptLoadError
func NewSarin(
	ctx context.Context,
	methods []string,
	requestURL *url.URL,
	timeout time.Duration,
	workers uint,
	totalRequests *uint64,
	totalDuration *time.Duration,
	quiet bool,
	skipCertVerify bool,
	params types.Params,
	headers types.Headers,
	cookies types.Cookies,
	bodies []string,
	proxies types.Proxies,
	values []string,
	collectStats bool,
	dryRun bool,
	luaScripts []string,
	jsScripts []string,
) (*sarin, error) {
	if workers == 0 {
		workers = 1
	}

	hostClients, err := newHostClients(ctx, timeout, proxies, workers, requestURL, skipCertVerify)
	if err != nil {
		return nil, err
	}

	// Load script sources
	luaSources, err := script.LoadSources(ctx, luaScripts, script.EngineTypeLua)
	if err != nil {
		return nil, err
	}

	jsSources, err := script.LoadSources(ctx, jsScripts, script.EngineTypeJavaScript)
	if err != nil {
		return nil, err
	}

	scriptChain := script.NewChain(luaSources, jsSources)

	srn := &sarin{
		workers:        workers,
		requestURL:     requestURL,
		methods:        methods,
		params:         params,
		headers:        headers,
		cookies:        cookies,
		bodies:         bodies,
		totalRequests:  totalRequests,
		totalDuration:  totalDuration,
		timeout:        timeout,
		quiet:          quiet,
		skipCertVerify: skipCertVerify,
		values:         values,
		collectStats:   collectStats,
		dryRun:         dryRun,
		hostClients:    hostClients,
		fileCache:      NewFileCache(time.Second * 10),
		scriptChain:    scriptChain,
	}

	if collectStats {
		srn.responses = NewSarinResponseData(uint32(100))
	}

	return srn, nil
}

func (q sarin) GetResponses() *SarinResponseData {
	return q.responses
}

func (q sarin) Start(ctx context.Context) {
	jobsCtx, jobsCancel := context.WithCancel(ctx)

	var workersWG sync.WaitGroup
	jobsCh := make(chan struct{}, max(q.workers, 1))

	var counter atomic.Uint64

	totalRequests := uint64(0)
	if q.totalRequests != nil {
		totalRequests = *q.totalRequests
	}

	var streamCtx context.Context
	var streamCancel context.CancelFunc
	var streamCh chan struct{}
	var messageChannel chan runtimeMessage
	var sendMessage messageSender

	if q.quiet {
		sendMessage = func(level runtimeMessageLevel, text string) {}
	} else {
		streamCtx, streamCancel = context.WithCancel(context.Background())
		defer streamCancel()
		streamCh = make(chan struct{})
		messageChannel = make(chan runtimeMessage, max(q.workers, 1))
		sendMessage = func(level runtimeMessageLevel, text string) {
			messageChannel <- runtimeMessage{
				timestamp: time.Now(),
				level:     level,
				text:      text,
			}
		}
	}

	// Start workers
	q.startWorkers(&workersWG, jobsCh, q.hostClients, &counter, sendMessage)

	if !q.quiet {
		// Start streaming to terminal
		//nolint:contextcheck // streamCtx must remain active until all workers complete to ensure all collected data is streamed
		go q.streamProgress(streamCtx, jobsCancel, streamCh, totalRequests, &counter, messageChannel)
	}

	// Setup duration-based cancellation
	q.setupDurationTimeout(ctx, jobsCancel)
	// Distribute jobs to workers.
	// This blocks until all jobs are sent or the context is canceled.
	q.sendJobs(jobsCtx, jobsCh)

	// Close the jobs channel so workers stop after completing their current job
	close(jobsCh)
	// Wait until all workers stopped
	workersWG.Wait()
	if messageChannel != nil {
		close(messageChannel)
	}

	if !q.quiet {
		// Stop the progress streaming
		streamCancel()
		// Wait until progress streaming has completely stopped
		<-streamCh
	}
}

func (q sarin) Worker(
	jobs <-chan struct{},
	hostClientGenerator HostClientGenerator,
	counter *atomic.Uint64,
	sendMessage messageSender,
) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	// Create script transformer for this worker (engines are not thread-safe)
	// Scripts are pre-validated in NewSarin, so this should not fail
	var scriptTransformer *script.Transformer
	if !q.scriptChain.IsEmpty() {
		var err error
		scriptTransformer, err = q.scriptChain.NewTransformer()
		if err != nil {
			panic(err)
		}
		defer scriptTransformer.Close()
	}

	requestGenerator, isDynamic := NewRequestGenerator(
		q.methods, q.requestURL, q.params, q.headers, q.cookies, q.bodies, q.values, q.fileCache, scriptTransformer,
	)

	if q.dryRun {
		switch {
		case q.collectStats && isDynamic:
			q.workerDryRunStatsWithDynamic(jobs, req, requestGenerator, counter, sendMessage)
		case q.collectStats && !isDynamic:
			q.workerDryRunStatsWithStatic(jobs, req, requestGenerator, counter, sendMessage)
		case !q.collectStats && isDynamic:
			q.workerDryRunNoStatsWithDynamic(jobs, req, requestGenerator, counter, sendMessage)
		default:
			q.workerDryRunNoStatsWithStatic(jobs, req, requestGenerator, counter, sendMessage)
		}
	} else {
		switch {
		case q.collectStats && isDynamic:
			q.workerStatsWithDynamic(jobs, req, resp, requestGenerator, hostClientGenerator, counter, sendMessage)
		case q.collectStats && !isDynamic:
			q.workerStatsWithStatic(jobs, req, resp, requestGenerator, hostClientGenerator, counter, sendMessage)
		case !q.collectStats && isDynamic:
			q.workerNoStatsWithDynamic(jobs, req, resp, requestGenerator, hostClientGenerator, counter, sendMessage)
		default:
			q.workerNoStatsWithStatic(jobs, req, resp, requestGenerator, hostClientGenerator, counter, sendMessage)
		}
	}
}

func (q sarin) workerStatsWithDynamic(
	jobs <-chan struct{},
	req *fasthttp.Request,
	resp *fasthttp.Response,
	requestGenerator RequestGenerator,
	hostClientGenerator HostClientGenerator,
	counter *atomic.Uint64,
	sendMessage messageSender,
) {
	for range jobs {
		req.Reset()
		resp.Reset()

		if err := requestGenerator(req); err != nil {
			q.responses.Add(err.Error(), 0)
			sendMessage(runtimeMessageLevelError, err.Error())
			counter.Add(1)
			continue
		}

		startTime := time.Now()
		err := hostClientGenerator().DoTimeout(req, resp, q.timeout)
		if err != nil {
			q.responses.Add(err.Error(), time.Since(startTime))
		} else {
			q.responses.Add(statusCodeToString(resp.StatusCode()), time.Since(startTime))
		}
		counter.Add(1)
	}
}

func (q sarin) workerStatsWithStatic(
	jobs <-chan struct{},
	req *fasthttp.Request,
	resp *fasthttp.Response,
	requestGenerator RequestGenerator,
	hostClientGenerator HostClientGenerator,
	counter *atomic.Uint64,
	sendMessage messageSender,
) {
	if err := requestGenerator(req); err != nil {
		// Static request generation failed - record all jobs as errors
		for range jobs {
			q.responses.Add(err.Error(), 0)
			sendMessage(runtimeMessageLevelError, err.Error())
			counter.Add(1)
		}
		return
	}

	for range jobs {
		resp.Reset()

		startTime := time.Now()
		err := hostClientGenerator().DoTimeout(req, resp, q.timeout)
		if err != nil {
			q.responses.Add(err.Error(), time.Since(startTime))
		} else {
			q.responses.Add(statusCodeToString(resp.StatusCode()), time.Since(startTime))
		}
		counter.Add(1)
	}
}

func (q sarin) workerNoStatsWithDynamic(
	jobs <-chan struct{},
	req *fasthttp.Request,
	resp *fasthttp.Response,
	requestGenerator RequestGenerator,
	hostClientGenerator HostClientGenerator,
	counter *atomic.Uint64,
	sendMessage messageSender,
) {
	for range jobs {
		req.Reset()
		resp.Reset()
		if err := requestGenerator(req); err != nil {
			sendMessage(runtimeMessageLevelError, err.Error())
			counter.Add(1)
			continue
		}
		_ = hostClientGenerator().DoTimeout(req, resp, q.timeout)
		counter.Add(1)
	}
}

func (q sarin) workerNoStatsWithStatic(
	jobs <-chan struct{},
	req *fasthttp.Request,
	resp *fasthttp.Response,
	requestGenerator RequestGenerator,
	hostClientGenerator HostClientGenerator,
	counter *atomic.Uint64,
	sendMessage messageSender,
) {
	if err := requestGenerator(req); err != nil {
		sendMessage(runtimeMessageLevelError, err.Error())

		// Static request generation failed - just count the jobs without sending
		for range jobs {
			counter.Add(1)
		}
		return
	}

	for range jobs {
		resp.Reset()
		_ = hostClientGenerator().DoTimeout(req, resp, q.timeout)
		counter.Add(1)
	}
}

const dryRunResponseKey = "dry-run"

// statusCodeStrings contains pre-computed string representations for HTTP status codes 100-599.
var statusCodeStrings = func() map[int]string {
	m := make(map[int]string, 500)
	for i := 100; i < 600; i++ {
		m[i] = strconv.Itoa(i)
	}
	return m
}()

// statusCodeToString returns a string representation of the HTTP status code.
// Uses a pre-computed map for codes 100-599, falls back to strconv.Itoa for others.
func statusCodeToString(code int) string {
	if s, ok := statusCodeStrings[code]; ok {
		return s
	}
	return strconv.Itoa(code)
}

func (q sarin) workerDryRunStatsWithDynamic(
	jobs <-chan struct{},
	req *fasthttp.Request,
	requestGenerator RequestGenerator,
	counter *atomic.Uint64,
	sendMessage messageSender,
) {
	for range jobs {
		req.Reset()
		startTime := time.Now()
		if err := requestGenerator(req); err != nil {
			q.responses.Add(err.Error(), time.Since(startTime))
			sendMessage(runtimeMessageLevelError, err.Error())
			counter.Add(1)
			continue
		}
		q.responses.Add(dryRunResponseKey, time.Since(startTime))
		counter.Add(1)
	}
}

func (q sarin) workerDryRunStatsWithStatic(
	jobs <-chan struct{},
	req *fasthttp.Request,
	requestGenerator RequestGenerator,
	counter *atomic.Uint64,
	sendMessage messageSender,
) {
	if err := requestGenerator(req); err != nil {
		// Static request generation failed - record all jobs as errors
		for range jobs {
			q.responses.Add(err.Error(), 0)
			sendMessage(runtimeMessageLevelError, err.Error())
			counter.Add(1)
		}
		return
	}

	for range jobs {
		q.responses.Add(dryRunResponseKey, 0)
		counter.Add(1)
	}
}

func (q sarin) workerDryRunNoStatsWithDynamic(
	jobs <-chan struct{},
	req *fasthttp.Request,
	requestGenerator RequestGenerator,
	counter *atomic.Uint64,
	sendMessage messageSender,
) {
	for range jobs {
		req.Reset()
		if err := requestGenerator(req); err != nil {
			sendMessage(runtimeMessageLevelError, err.Error())
		}
		counter.Add(1)
	}
}

func (q sarin) workerDryRunNoStatsWithStatic(
	jobs <-chan struct{},
	req *fasthttp.Request,
	requestGenerator RequestGenerator,
	counter *atomic.Uint64,
	sendMessage messageSender,
) {
	if err := requestGenerator(req); err != nil {
		sendMessage(runtimeMessageLevelError, err.Error())
	}

	for range jobs {
		counter.Add(1)
	}
}

// newHostClients initializes HTTP clients for the given configuration.
// It can return the following errors:
// - types.ProxyDialError
func newHostClients(
	ctx context.Context,
	timeout time.Duration,
	proxies types.Proxies,
	workers uint,
	requestURL *url.URL,
	skipCertVerify bool,
) ([]*fasthttp.HostClient, error) {
	proxiesRaw := make([]url.URL, len(proxies))
	for i, proxy := range proxies {
		proxiesRaw[i] = url.URL(proxy)
	}

	maxConns := max(fasthttp.DefaultMaxConnsPerHost, workers)
	maxConns = ((maxConns * 50 / 100) + maxConns)
	return NewHostClients(
		ctx,
		timeout,
		proxiesRaw,
		maxConns,
		requestURL,
		skipCertVerify,
	)
}

func (q sarin) startWorkers(wg *sync.WaitGroup, jobs <-chan struct{}, hostClients []*fasthttp.HostClient, counter *atomic.Uint64, sendMessage messageSender) {
	for range max(q.workers, 1) {
		wg.Go(func() {
			q.Worker(jobs, NewHostClientGenerator(hostClients...), counter, sendMessage)
		})
	}
}

func (q sarin) setupDurationTimeout(ctx context.Context, cancel context.CancelFunc) {
	if q.totalDuration != nil {
		go func() {
			timer := time.NewTimer(*q.totalDuration)
			defer timer.Stop()
			select {
			case <-timer.C:
				cancel()
			case <-ctx.Done():
				// Context cancelled, cleanup
			}
		}()
	}
}

func (q sarin) sendJobs(ctx context.Context, jobs chan<- struct{}) {
	if q.totalRequests != nil && *q.totalRequests > 0 {
		for range *q.totalRequests {
			if ctx.Err() != nil {
				break
			}
			jobs <- struct{}{}
		}
	} else {
		for ctx.Err() == nil {
			jobs <- struct{}{}
		}
	}
}

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
	cancel     context.CancelFunc
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
			m.cancel()
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
		finalBuilder.WriteString(helpStyle.Render("Stopping..."))
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
	cancel     context.CancelFunc
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
			m.cancel()
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
			finalBuilder.WriteString(helpStyle.Render("Stopping..."))
		} else {
			finalBuilder.WriteString(helpStyle.Render("Press Ctrl+C to quit"))
		}
	}
	return finalBuilder.String()
}

func (q sarin) streamProgress(
	ctx context.Context,
	cancel context.CancelFunc,
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
			cancel:    cancel,
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
			cancel:    cancel,
			quit:      false,
		}

		program = tea.NewProgram(model)
	}

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
