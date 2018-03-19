// +build !go1.9

package pprofdo

import (
	"context"
)

func Do(ctx context.Context, labels []string, f func(context.Context)) {
	f(ctx)
}
