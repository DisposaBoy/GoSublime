// +build go1.9

package pprofdo

import (
	"context"
	"runtime/pprof"
)

func Do(ctx context.Context, labels []string, f func(context.Context)) {
	pprof.Do(ctx, pprof.Labels(labels...), f)
}
