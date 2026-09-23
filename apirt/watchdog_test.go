package apirt

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestReadWatchdog(t *testing.T) {
	const period = 1 * time.Second

	type TestCase struct {
		MessageCount  int  // the number of messages to generate
		ShouldTimeout bool // if true, we will attempt to trigger a timeout in the readWatchdog
	}

	cases := []TestCase{
		{
			MessageCount:  5,
			ShouldTimeout: false,
		},
		{
			MessageCount:  5,
			ShouldTimeout: true,
		},
		{
			MessageCount:  10,
			ShouldTimeout: false,
		},
		{
			MessageCount:  10,
			ShouldTimeout: true,
		},
	}

	for i := range cases {
		tc := &cases[i]
		name := fmt.Sprintf("%d", i)
		t.Run(name, func(t *testing.T) {

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var (
				inCh         = make(chan message)
				outCh        = make(chan message)
				wg           = sync.WaitGroup{}
				cancelCalled atomic.Bool
			)

			// push messages into inCh
			wg.Add(1)
			go func() {
				defer wg.Done()

				for i := 0; i < tc.MessageCount; i++ {
					inCh <- message{
						Type:    dataMessage,
						Payload: binary.BigEndian.AppendUint64(nil, uint64(i)),
					}

					if tc.ShouldTimeout {
						time.Sleep(period + (250 * time.Millisecond))
					} else {
						time.Sleep(period / 2)
					}
				}

				// push the eof message
				t.Logf("sending eof")
				inCh <- message{Type: eofMessage}
			}()

			// filter messages through the watchdog
			startReadWatchdog(ctx, period, inCh, outCh, func() {
				cancelCalled.Store(true)
			})

			// collect messages
			var (
				collectedMessages []message
				periodExceeded    bool
			)

			wg.Add(1)
			go func() {
				defer wg.Done()

				var lastMessage time.Time

				for msg := range outCh {
					now := time.Now()

					if !lastMessage.IsZero() {
						sinceLastMessage := now.Sub(lastMessage)
						t.Logf("%s since last message", sinceLastMessage.String())

						if sinceLastMessage > period {
							periodExceeded = true
						}
					}

					lastMessage = now

					collectedMessages = append(collectedMessages, msg)
				}
			}()

			// wait for goroutines to exit
			wg.Wait()

			// examine collected messages
			t.Logf("periodExceeded = %t", periodExceeded)
			t.Logf("cancelCalled = %t", cancelCalled.Load())

			if len(collectedMessages) != tc.MessageCount {
				t.Fatal("failed to collect all messages")
			}

			if periodExceeded && !cancelCalled.Load() {
				t.Fatal("period was exceeded, but cancel was not called")
			}
		})
	}
}
