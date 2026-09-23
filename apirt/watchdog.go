package apirt

import (
	"context"
	"time"

	"github.com/mrdude/pcc-common"
)

// startReadWatchdog starts a goroutine that:
// 1. Reads messages from inCh and writes them to outCh
// 2. Calls cancel() if a message isn't received from inCh every period seconds
func startReadWatchdog(ctx context.Context, period time.Duration, inCh, outCh chan message, cancel func()) {
	go func() {
		defer close(outCh)

		t := time.NewTicker(period)
		defer t.Stop()

		logger := pcommon.GetLogger(ctx)

		for {
			select {
			case <-ctx.Done():
				if err := ctx.Err(); err != nil {
					return
				}
			case <-t.C:
				// timeout occurred -- cancel the channel.
				// we'll exit the loop when the channel closes
				logger.Debug("timeout occurred; closing channel")
				cancel()
			case v, ok := <-inCh:
				if ok {
					switch v.Type {
					case dataMessage:
						outCh <- v
					case eofMessage:
						// time to bail out
						return
					case keepAliveMessage:
					}
					t.Reset(period)
				} else {
					// channel is closed -- bail out
					logger.Debug("upstream closed the channel -- bail out")
					return
				}
			}
		}
	}()
}
