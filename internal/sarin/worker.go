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
var statusCodeStrings = func() [500]string {
	var codes [500]string
	for i := range codes {
		codes[i] = strconv.Itoa(i + 100)
	}
	return codes
}()

// statusCodeToString returns a string representation of the HTTP status code.
// Uses a pre-computed table for codes 100-599, falls back to strconv.Itoa for others.
func statusCodeToString(code int) string {
	if i := code - 100; i >= 0 && i < len(statusCodeStrings) {
		return statusCodeStrings[i]
	}
	return strconv.Itoa(code)
}

func (s sarin) Worker(
	jobs <-chan struct{},
	hostClientGenerator HostClientGenerator,
	counter *atomic.Uint64,
	sendLog runtimeLogger,
	sendRespLog respLogger,
) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	// Create script transformer for this worker (engines are not thread-safe)
	// Scripts are pre-validated in NewSarin, so this should not fail
	var scriptTransformer *script.Transformer
	if !s.scriptChain.IsEmpty() {
		var err error
		scriptTransformer, err = s.scriptChain.NewTransformer()
		if err != nil {
			panic(err)
		}
		defer scriptTransformer.Close()
	}

	requestGenerator, isDynamic := NewRequestGenerator(
		s.methods, s.requestURL, s.params, s.headers, s.cookies, s.bodies, s.values, s.fileCache, scriptTransformer,
	)

	if s.dryRun {
		switch {
		case s.collectStats && isDynamic:
			s.workerDryRunStatsWithDynamic(jobs, req, requestGenerator, counter, sendLog)
		case s.collectStats && !isDynamic:
			s.workerDryRunStatsWithStatic(jobs, req, requestGenerator, counter, sendLog)
		case !s.collectStats && isDynamic:
			s.workerDryRunNoStatsWithDynamic(jobs, req, requestGenerator, counter, sendLog)
		default:
			s.workerDryRunNoStatsWithStatic(jobs, req, requestGenerator, counter, sendLog)
		}
	} else {
		switch {
		case s.collectStats && isDynamic:
			s.workerStatsWithDynamic(jobs, req, resp, requestGenerator, hostClientGenerator, counter, sendLog, sendRespLog)
		case s.collectStats && !isDynamic:
			s.workerStatsWithStatic(jobs, req, resp, requestGenerator, hostClientGenerator, counter, sendLog, sendRespLog)
		case !s.collectStats && isDynamic:
			s.workerNoStatsWithDynamic(jobs, req, resp, requestGenerator, hostClientGenerator, counter, sendLog, sendRespLog)
		default:
			s.workerNoStatsWithStatic(jobs, req, resp, requestGenerator, hostClientGenerator, counter, sendLog, sendRespLog)
		}
	}
}

func (s sarin) workerStatsWithDynamic(
	jobs <-chan struct{},
	req *fasthttp.Request,
	resp *fasthttp.Response,
	requestGenerator RequestGenerator,
	hostClientGenerator HostClientGenerator,
	counter *atomic.Uint64,
	sendLog runtimeLogger,
	sendRespLog respLogger,
) {
	for range jobs {
		req.Reset()

		if err := requestGenerator(req); err != nil {
			s.responses.Add(err.Error(), 0)
			sendLog(runtimeLogLevelError, err.Error())
			counter.Add(1)
			continue
		}

		startTime := time.Now()
		err := hostClientGenerator().DoTimeout(req, resp, s.timeout)
		respDuration := time.Since(startTime)

		if err != nil {
			s.responses.Add(err.Error(), respDuration)
		} else {
			s.responses.Add(statusCodeToString(resp.StatusCode()), respDuration)
			sendRespLog(respDuration, resp)
		}
		counter.Add(1)
	}
}

func (s sarin) workerStatsWithStatic(
	jobs <-chan struct{},
	req *fasthttp.Request,
	resp *fasthttp.Response,
	requestGenerator RequestGenerator,
	hostClientGenerator HostClientGenerator,
	counter *atomic.Uint64,
	sendLog runtimeLogger,
	sendRespLog respLogger,
) {
	if err := requestGenerator(req); err != nil {
		// Static request generation failed - record all jobs as errors
		for range jobs {
			s.responses.Add(err.Error(), 0)
			sendLog(runtimeLogLevelError, err.Error())
			counter.Add(1)
		}
		return
	}

	for range jobs {
		startTime := time.Now()
		err := hostClientGenerator().DoTimeout(req, resp, s.timeout)
		respDuration := time.Since(startTime)
		if err != nil {
			s.responses.Add(err.Error(), respDuration)
		} else {
			s.responses.Add(statusCodeToString(resp.StatusCode()), respDuration)
			sendRespLog(respDuration, resp)
		}
		counter.Add(1)
	}
}

func (s sarin) workerNoStatsWithDynamic(
	jobs <-chan struct{},
	req *fasthttp.Request,
	resp *fasthttp.Response,
	requestGenerator RequestGenerator,
	hostClientGenerator HostClientGenerator,
	counter *atomic.Uint64,
	sendLog runtimeLogger,
	sendRespLog respLogger,
) {
	for range jobs {
		req.Reset()
		if err := requestGenerator(req); err != nil {
			sendLog(runtimeLogLevelError, err.Error())
			counter.Add(1)
			continue
		}
		startTime := time.Now()
		err := hostClientGenerator().DoTimeout(req, resp, s.timeout)
		if err == nil {
			sendRespLog(time.Since(startTime), resp)
		}
		counter.Add(1)
	}
}

func (s sarin) workerNoStatsWithStatic(
	jobs <-chan struct{},
	req *fasthttp.Request,
	resp *fasthttp.Response,
	requestGenerator RequestGenerator,
	hostClientGenerator HostClientGenerator,
	counter *atomic.Uint64,
	sendLog runtimeLogger,
	sendRespLog respLogger,
) {
	if err := requestGenerator(req); err != nil {
		sendLog(runtimeLogLevelError, err.Error())

		// Static request generation failed - just count the jobs without sending
		for range jobs {
			counter.Add(1)
		}
		return
	}

	for range jobs {
		startTime := time.Now()
		err := hostClientGenerator().DoTimeout(req, resp, s.timeout)
		if err == nil {
			sendRespLog(time.Since(startTime), resp)
		}
		counter.Add(1)
	}
}

func (s sarin) workerDryRunStatsWithDynamic(
	jobs <-chan struct{},
	req *fasthttp.Request,
	requestGenerator RequestGenerator,
	counter *atomic.Uint64,
	sendLog runtimeLogger,
) {
	for range jobs {
		req.Reset()
		startTime := time.Now()
		if err := requestGenerator(req); err != nil {
			s.responses.Add(err.Error(), time.Since(startTime))
			sendLog(runtimeLogLevelError, err.Error())
			counter.Add(1)
			continue
		}
		s.responses.Add(dryRunResponseKey, time.Since(startTime))
		counter.Add(1)
	}
}

func (s sarin) workerDryRunStatsWithStatic(
	jobs <-chan struct{},
	req *fasthttp.Request,
	requestGenerator RequestGenerator,
	counter *atomic.Uint64,
	sendLog runtimeLogger,
) {
	if err := requestGenerator(req); err != nil {
		// Static request generation failed - record all jobs as errors
		for range jobs {
			s.responses.Add(err.Error(), 0)
			sendLog(runtimeLogLevelError, err.Error())
			counter.Add(1)
		}
		return
	}

	for range jobs {
		s.responses.Add(dryRunResponseKey, 0)
		counter.Add(1)
	}
}

func (s sarin) workerDryRunNoStatsWithDynamic(
	jobs <-chan struct{},
	req *fasthttp.Request,
	requestGenerator RequestGenerator,
	counter *atomic.Uint64,
	sendLog runtimeLogger,
) {
	for range jobs {
		req.Reset()
		if err := requestGenerator(req); err != nil {
			sendLog(runtimeLogLevelError, err.Error())
		}
		counter.Add(1)
	}
}

func (s sarin) workerDryRunNoStatsWithStatic(
	jobs <-chan struct{},
	req *fasthttp.Request,
	requestGenerator RequestGenerator,
	counter *atomic.Uint64,
	sendLog runtimeLogger,
) {
	if err := requestGenerator(req); err != nil {
		sendLog(runtimeLogLevelError, err.Error())
	}

	for range jobs {
		counter.Add(1)
	}
}
