package sarin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/valyala/fasthttp"
	"go.aykhans.me/sarin/internal/script"
	"go.aykhans.me/sarin/internal/types"
)

type runtimeLogLevel uint8

const (
	runtimeLogLevelInfo runtimeLogLevel = iota
	runtimeLogLevelError
)

type runtimeLog struct {
	timestamp time.Time
	level     runtimeLogLevel
	text      string
}

type runtimeLogger func(level runtimeLogLevel, text string)

// SplitLogLevels parses a comma-separated log-level string into a normalized,
// deduplicated slice of level tokens (lowercased and trimmed, empties dropped).
func SplitLogLevels(levels string) []string {
	var out []string
	seen := make(map[string]bool)
	for part := range strings.SplitSeq(levels, ",") {
		token := strings.ToLower(strings.TrimSpace(part))
		if token == "" || seen[token] {
			continue
		}
		seen[token] = true
		out = append(out, token)
	}
	return out
}

// respLogger logs a single completed response.
type respLogger func(duration time.Duration, resp *fasthttp.Response)

func noopLog(runtimeLogLevel, string)               {}
func noopRespLog(time.Duration, *fasthttp.Response) {}

// gateSendLog wraps emit with level filtering decided once from the enabled
// levels. It stays general (any level filters correctly) while avoiding a
// per-log check when both levels are on and any work at all when both are off.
func gateSendLog(logInfo, logError bool, emit func(level runtimeLogLevel, text string)) runtimeLogger {
	switch {
	case logInfo && logError:
		return emit
	case logInfo:
		return func(level runtimeLogLevel, text string) {
			if level == runtimeLogLevelInfo {
				emit(level, text)
			}
		}
	case logError:
		return func(level runtimeLogLevel, text string) {
			if level == runtimeLogLevelError {
				emit(level, text)
			}
		}
	default:
		return noopLog
	}
}

// respBodySnippetLen bounds how many bytes of the body the compact (TUI)
// rendering shows.
const respBodySnippetLen = 100

// formatRuntimeLogLine renders a runtime log as a plain (unstyled) line for
// stderr output. ANSI styling is left to the TUI.
func formatRuntimeLogLine(timestamp time.Time, level runtimeLogLevel, text string) string {
	levelStr := "ERROR"
	if level == runtimeLogLevelInfo {
		levelStr = "INFO"
	}
	return "[" + timestamp.Format("15:04:05") + "] " + levelStr + ": " + text
}

// respToLog renders a response as a compact one-line summary for the TUI log box
// (the "[time] INFO:" prefix is added by the TUI).
func respToLog(duration time.Duration, resp *fasthttp.Response) string {
	var sb strings.Builder
	sb.WriteString(statusCodeToString(resp.StatusCode()))
	sb.WriteString(" ")
	sb.WriteString(Duration(duration).String())
	if snippet := bodySnippet(resp.Body(), respBodySnippetLen); snippet != "" {
		sb.WriteString(" | ")
		sb.WriteString(snippet)
	}
	return sb.String()
}

type respLogEntry struct {
	Status   int                 `json:"status"`
	Duration string              `json:"duration"`
	Headers  map[string][]string `json:"headers,omitempty"`
	Body     string              `json:"body,omitempty"`
}

// respToLogJSON renders a response as a single, self-contained JSON line so the
// stderr stream stays valid NDJSON (pipeable to jq or any consumer).
func respToLogJSON(duration time.Duration, resp *fasthttp.Response) string {
	entry := respLogEntry{
		Status:   resp.StatusCode(),
		Duration: Duration(duration).String(),
		Headers:  collectRespHeaders(resp),
		Body:     string(resp.Body()),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return `{"error":"failed to marshal response log"}`
	}
	return string(data)
}

func collectRespHeaders(resp *fasthttp.Response) map[string][]string {
	headers := make(map[string][]string)
	for key, value := range resp.Header.All() {
		k := string(key)
		headers[k] = append(headers[k], string(value))
	}
	return headers
}

// bodySnippet returns the first maxLen bytes of body collapsed onto a single
// line (control characters replaced with spaces), with an ellipsis when truncated.
func bodySnippet(body []byte, maxLen int) string {
	if len(body) == 0 {
		return ""
	}

	truncated := false
	if len(body) > maxLen {
		body = body[:maxLen]
		truncated = true
	}

	s := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, string(body))

	if truncated {
		s += "..."
	}
	return s
}

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
	showProgress   bool
	skipCertVerify bool
	values         []string
	collectStats   bool
	dryRun         bool
	logInfo        bool
	logError       bool
	logFile        string

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
	showProgress bool,
	skipCertVerify bool,
	params types.Params,
	headers types.Headers,
	cookies types.Cookies,
	bodies []string,
	proxies types.Proxies,
	values []string,
	collectStats bool,
	dryRun bool,
	logLevel string,
	logFile string,
	luaScripts []string,
	jsScripts []string,
) (*sarin, error) {
	if workers == 0 {
		workers = 1
	}

	// Resolve which log levels are enabled once, up front.
	var logInfo, logError bool
	for _, level := range SplitLogLevels(logLevel) {
		switch level {
		case "info":
			logInfo = true
		case "error":
			logError = true
		}
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
		showProgress:   showProgress,
		skipCertVerify: skipCertVerify,
		values:         values,
		collectStats:   collectStats,
		dryRun:         dryRun,
		logInfo:        logInfo,
		logError:       logError,
		logFile:        logFile,
		hostClients:    hostClients,
		fileCache:      NewFileCache(time.Second * 10),
		scriptChain:    scriptChain,
	}

	if collectStats {
		srn.responses = NewSarinResponseData(uint32(100))
	}

	return srn, nil
}

func (s sarin) GetResponses() *SarinResponseData {
	return s.responses
}

func (s sarin) Start(ctx context.Context, stopCtrl *StopController) {
	jobsCtx, jobsCancel := context.WithCancel(ctx)

	var workersWG sync.WaitGroup
	jobsCh := make(chan struct{}, max(s.workers, 1))

	var counter atomic.Uint64

	totalRequests := uint64(0)
	if s.totalRequests != nil {
		totalRequests = *s.totalRequests
	}

	onTerminal := term.IsTerminal(os.Stdout.Fd())
	// The progress bar needs an interactive terminal to render.
	showProgressBar := s.showProgress && onTerminal
	// The bubbletea TUI hosts the bar and/or the live log box, so it runs whenever
	// either has something to show: the bar, or logs that would land in the box
	// (i.e. logs are enabled and not redirected to a file).
	runTUI := showProgressBar || (onTerminal && (s.logInfo || s.logError) && s.logFile == "")

	// Open the log file up front, before registering any defer, so a bad path
	// exits cleanly.
	var logFile *os.File
	if s.logFile != "" {
		f, err := os.OpenFile(s.logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to open log file "+s.logFile+": "+err.Error())
			os.Exit(1)
		}
		defer f.Close() //nolint:errcheck
		logFile = f
	}

	var (
		streamCtx     context.Context
		streamCancel  context.CancelFunc
		streamCh      chan struct{}
		tuiLogChannel chan runtimeLog
	)
	if runTUI {
		streamCtx, streamCancel = context.WithCancel(context.Background())
		defer streamCancel()
		streamCh = make(chan struct{})
		tuiLogChannel = make(chan runtimeLog, max(s.workers, 1))
	}

	// Route logs: to the file if given, else to the TUI log box while it runs,
	// otherwise to stderr.
	var (
		sendLog     runtimeLogger
		sendRespLog respLogger
	)
	switch {
	case logFile != nil:
		sendLog, sendRespLog = s.newWriterLog(logFile)
	case runTUI:
		sendLog, sendRespLog = s.newChannelLog(tuiLogChannel)
	default:
		sendLog, sendRespLog = s.newWriterLog(os.Stderr)
	}

	// Start workers
	s.startWorkers(&workersWG, jobsCh, s.hostClients, &counter, sendLog, sendRespLog)

	if runTUI {
		//nolint:contextcheck // streamCtx must remain active until all workers complete to ensure all collected data is streamed
		go s.streamProgress(streamCtx, stopCtrl, streamCh, totalRequests, &counter, tuiLogChannel, showProgressBar)
	}

	// Setup duration-based cancellation
	s.setupDurationTimeout(ctx, jobsCancel)
	// Distribute jobs to workers.
	// This blocks until all jobs are sent or the context is canceled.
	s.sendJobs(jobsCtx, jobsCh)

	// Close the jobs channel so workers stop after completing their current job
	close(jobsCh)
	// Wait until all workers stopped
	workersWG.Wait()
	if tuiLogChannel != nil {
		close(tuiLogChannel)
	}

	if runTUI {
		// Stop the progress streaming
		streamCancel()
		// Wait until progress streaming has completely stopped
		<-streamCh
	}
}

// newWriterLog builds the loggers that write formatted lines to w (a log file or
// stderr). sendLog stays general (it filters by each log's level); sendRespLog
// only ever emits info, so its decision is baked once into a no-op when off.
func (s sarin) newWriterLog(w io.Writer) (runtimeLogger, respLogger) {
	// log.Logger serializes writes with its own mutex, so concurrent workers
	// won't interleave lines.
	logger := log.New(w, "", 0)

	sendLog := gateSendLog(s.logInfo, s.logError, func(level runtimeLogLevel, text string) {
		logger.Println(formatRuntimeLogLine(time.Now(), level, text))
	})

	var sendRespLog respLogger = noopRespLog
	if s.logInfo {
		sendRespLog = func(duration time.Duration, resp *fasthttp.Response) {
			logger.Println(formatRuntimeLogLine(time.Now(), runtimeLogLevelInfo, respToLogJSON(duration, resp)))
		}
	}

	return sendLog, sendRespLog
}

// newChannelLog builds the loggers that feed the TUI log box through ch, with the
// same gating as newWriterLog.
func (s sarin) newChannelLog(ch chan<- runtimeLog) (runtimeLogger, respLogger) {
	sendLog := gateSendLog(s.logInfo, s.logError, func(level runtimeLogLevel, text string) {
		ch <- runtimeLog{timestamp: time.Now(), level: level, text: text}
	})

	var sendRespLog respLogger = noopRespLog
	if s.logInfo {
		sendRespLog = func(duration time.Duration, resp *fasthttp.Response) {
			ch <- runtimeLog{timestamp: time.Now(), level: runtimeLogLevelInfo, text: respToLog(duration, resp)}
		}
	}

	return sendLog, sendRespLog
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

	return NewHostClients(
		ctx,
		timeout,
		proxiesRaw,
		workers,
		requestURL,
		skipCertVerify,
	)
}

func (s sarin) startWorkers(wg *sync.WaitGroup, jobs <-chan struct{}, hostClients []*fasthttp.HostClient, counter *atomic.Uint64, sendLog runtimeLogger, sendRespLog respLogger) {
	for range max(s.workers, 1) {
		wg.Go(func() {
			s.Worker(jobs, NewHostClientGenerator(hostClients...), counter, sendLog, sendRespLog)
		})
	}
}

func (s sarin) setupDurationTimeout(ctx context.Context, cancel context.CancelFunc) {
	if s.totalDuration != nil {
		go func() {
			timer := time.NewTimer(*s.totalDuration)
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

func (s sarin) sendJobs(ctx context.Context, jobs chan<- struct{}) {
	if s.totalRequests != nil && *s.totalRequests > 0 {
		for range *s.totalRequests {
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
