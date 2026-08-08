package sarin

import (
	"context"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
	"go.aykhans.me/sarin/internal/types"
)

// BenchmarkRequestGenerator measures the cost of building one request, which is
// the work every worker repeats before each send.
func BenchmarkRequestGenerator(b *testing.B) {
	run := func(b *testing.B, params types.Params, headers types.Headers, cookies types.Cookies, bodies []string) {
		b.Helper()

		requestURL, err := url.Parse("http://127.0.0.1:8080/api/v1/items")
		if err != nil {
			b.Fatal(err)
		}
		generator, _ := NewRequestGenerator(
			[]string{"POST"}, requestURL, params, headers, cookies, bodies, nil,
			NewFileCache(time.Second), nil,
		)

		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)

		b.ReportAllocs()
		for b.Loop() {
			req.Reset()
			if err := generator(req); err != nil {
				b.Fatal(err)
			}
		}
	}

	b.Run("static", func(b *testing.B) {
		run(b,
			types.Params{{Key: "page", Value: []string{"2"}}},
			types.Headers{
				{Key: "User-Agent", Value: []string{"sarin"}},
				{Key: "Accept", Value: []string{"application/json"}},
			},
			types.Cookies{{Key: "session", Value: []string{"xyz"}}},
			[]string{`{"name":"item"}`},
		)
	})

	b.Run("templated", func(b *testing.B) {
		run(b,
			types.Params{{Key: "q", Value: []string{"{{ fakeit_Word }}"}}},
			types.Headers{
				{Key: "User-Agent", Value: []string{"sarin"}},
				{Key: "X-Trace", Value: []string{"{{ fakeit_UUID }}"}},
			},
			types.Cookies{{Key: "session", Value: []string{"{{ fakeit_UUID }}"}}},
			[]string{`{"name":"{{ fakeit_Name }}"}`},
		)
	})
}

// BenchmarkDryRun measures a whole run end to end with the network taken out, so
// what it reports is sarin's own per-request overhead: handing work to workers,
// building the request and recording the result.
func BenchmarkDryRun(b *testing.B) {
	for _, workers := range []uint{16, 128, 512} {
		b.Run("workers="+strconv.FormatUint(uint64(workers), 10), func(b *testing.B) {
			requestURL, err := url.Parse("http://127.0.0.1:8080/api/v1/items")
			if err != nil {
				b.Fatal(err)
			}

			total := uint64(b.N)
			srn, err := NewSarin(
				b.Context(),
				[]string{"GET"}, requestURL, 5*time.Second, workers, &total, nil,
				false, false,
				types.Params{{Key: "page", Value: []string{"2"}}},
				types.Headers{{Key: "User-Agent", Value: []string{"sarin"}}},
				types.Cookies{{Key: "session", Value: []string{"xyz"}}},
				[]string{`{"name":"item"}`},
				nil, nil,
				true, // collect stats
				true, // dry run
				"", "", nil, nil,
			)
			if err != nil {
				b.Fatal(err)
			}

			ctx, cancel := context.WithCancel(b.Context())
			defer cancel()

			b.ResetTimer()
			srn.Start(ctx, NewStopController(cancel))
			b.StopTimer()
		})
	}
}
