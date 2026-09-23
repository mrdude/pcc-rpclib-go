package apitrace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	pcommon "github.com/mrdude/pcc-common"
	"github.com/mrdude/pcc-rpclib-go/apitrace/zipkin"
	"go.uber.org/zap"
)

const (
	maxBatchSize         = 240
	batchPeriod          = 5 * time.Second
	batchJitter          = 1 * time.Second
	batchSubmitTimeout   = 5 * time.Second
	batchSubmitBaseDelay = 2 * time.Second // TODO tune this
	batchSubmitMaxDelay  = batchSubmitTimeout
	zipkinUrl            = "https://tracing-ingest.home.prztl.com/api/v2/spans"
)

type zipkinForwarder struct {
	rawOut chan<- zipkin.Span
}

func StartZipkinForwarder(ctx context.Context) Forwarder {
	rawChan := make(chan zipkin.Span, 32)

	f := &zipkinForwarder{
		rawOut: rawChan,
	}

	// accept incoming zipkin.Span's, batch them, and submit them
	func(in <-chan zipkin.Span) {
		go func() {
			var (
				batch []zipkin.Span
				cl    = http.Client{}
				t     = time.NewTicker(batchPeriod + randDuration(batchJitter))
			)
			defer t.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case span := <-in:
					// add to the batch
					batch = append(batch, span)

					// trim batch if needed
					if trim := len(batch) - maxBatchSize; trim > 0 {
						batch = batch[trim:]
					}
				case <-t.C:
					if len(batch) > 0 {
						// submit the batch
						if err := submitBatchWithRetry(ctx, &cl, batch, 2); err != nil {
							pcommon.GetLogger(ctx).Error("failed to submit span batch",
								zap.Error(err))
						} else {
							pcommon.GetLogger(ctx).Debug("submitted span batch",
								zap.Int("batch.len", len(batch)),
							)

							// clear the batch
							batch = nil
						}
					}

					// add some jitter to the timer
					t.Reset(batchPeriod + randDuration(batchJitter))
				}
			}
		}()
	}(rawChan)

	return f
}

func randDuration(N time.Duration) time.Duration {
	X := rand.Int63n(int64(N))
	return time.Duration(X)
}

func (f *zipkinForwarder) SubmitSpan(s zipkin.Span) {
	if !s.IsSampled() {
		return
	}

	now := time.Now()
	s.Duration = zipkin.Duration{Duration: now.Sub(s.Ts.Time)}
	f.rawOut <- s.Clone()
}

func submitBatchWithRetry(ctx context.Context, cl *http.Client, batch []zipkin.Span, maxretries int) error {
	if maxretries <= 0 {
		panic(errors.New("maxretries must be >= 1"))
	}

	var (
		retry = 0
	)
	for {
		// attempt to submit batch
		err := submitBatch(ctx, cl, batch)
		if err == nil {
			return nil
		}

		// increment retry count and respect maxretries
		retry++
		if retry >= maxretries {
			return err
		}

		// handle fallback
		delay := pow(2, time.Duration(retry)) * batchSubmitBaseDelay
		if delay > batchSubmitMaxDelay {
			delay = batchSubmitMaxDelay
		}
		time.Sleep(delay)
	}
}

func submitBatch(ctx context.Context, cl *http.Client, batch []zipkin.Span) error {
	reqCtx, cancel := context.WithTimeout(ctx, batchSubmitTimeout)
	defer cancel()

	b, err := json.Marshal(batch)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(reqCtx, "POST", zipkinUrl, bytes.NewReader(b))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := cl.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if isSuccessful := resp.StatusCode >= 200 || resp.StatusCode <= 299; !isSuccessful {
		return fmt.Errorf("non-200 status code returned: %d", resp.StatusCode)
	}

	return nil
}

// pow returns "a to the b power" for two positive durations a, b
// will panic for negative a or b!
func pow(a, b time.Duration) time.Duration {
	if a < 0 || b < 0 {
		panic(errors.New("a,b > 0"))
	}

	switch b {
	case 0:
		return 1
	case 1:
		return a
	case 2:
		return a * a
	default:
		if b%2 == 0 {
			tmp := pow(a, b/2)
			return tmp * tmp
		} else {
			tmp := pow(a, (b-1)/2)
			return tmp * tmp * a
		}
	}
}
