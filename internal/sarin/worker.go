package sarin

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
	"go.aykhans.me/sarin/internal/script"
)

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
