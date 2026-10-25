package sarin

import (
	"context"
	"sync/atomic"
)

// jobSource hands out requests to the workers, with no queue in between.
type jobSource struct {
	ctx     context.Context //nolint:containedctx
	total   uint64
	claimed atomic.Uint64
}

// newJobSource limits the run to total requests, with no limit when total is 0.
func newJobSource(ctx context.Context, total uint64) *jobSource {
	return &jobSource{ctx: ctx, total: total}
}

// next claims one request. It reports false once the run is stopped or every request is claimed.
func (j *jobSource) next() bool {
	if j.ctx.Err() != nil {
		return false
	}
	return j.total == 0 || j.claimed.Add(1) <= j.total
}
