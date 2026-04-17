package sarin

import (
	"context"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/x/term"
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

func (q sarin) Start(ctx context.Context, stopCtrl *StopController) {
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

	if !q.quiet && !term.IsTerminal(os.Stdout.Fd()) {
		q.quiet = true
	}

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
		go q.streamProgress(streamCtx, stopCtrl, streamCh, totalRequests, &counter, messageChannel)
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
