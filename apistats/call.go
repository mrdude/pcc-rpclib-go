package apistats

import (
	"time"
)

type Call struct {
	StatusCode             int    // the status code of the response
	ApiName, OperationName string // the call key

	side     string        // server or client -- what "side" of the RPC is this?
	start    time.Time     // when did the call start
	duration time.Duration // how long did the call take?
}

func NewServerCall() *Call {
	return newCall("server")
}

func NewClientCall() *Call {
	return newCall("client")
}

func newCall(side string) *Call {
	return &Call{
		side:  side,
		start: time.Now(),
	}
}

// Submit submits the call to the metrics sink.
// the *Call object should not be used after Submit is called
func (call *Call) Submit() {
	// TODO externalize internal stats library, then uncomment this
	/*
		call.duration = time.Since(call.start)

		// submit latency metric
		stats.Distribution(&bfly.SchemaRef{
			Name: fmt.Sprintf("/rpc/%s/sec_latency", call.side),
			Keys: []string{
				"com.prztl.home.prpc",
				call.ApiName,
				call.OperationName,
			},
		}).Submit(call.duration.Seconds())

		// update request count
		reqCount := stats.Counter(&bfly.SchemaRef{
			Name: fmt.Sprintf("/rpc/%s/requests", call.side),
			Keys: []string{
				"com.prztl.home.prpc",
				call.ApiName,
				call.OperationName,
			},
		})
		reqCount.Increment()

		// update success/error/fault counter
		statusCodeType := "successes"
		if call.StatusCode >= 400 && call.StatusCode <= 499 {
			statusCodeType = "errors"
		} else if call.StatusCode >= 500 && call.StatusCode <= 599 {
			statusCodeType = "faults"
		}

		resultTypeCount := stats.Counter(&bfly.SchemaRef{
			Name: fmt.Sprintf("/rpc/%s/%s", call.side, statusCodeType),
			Keys: []string{
				"com.prztl.home.prpc",
				call.ApiName,
				call.OperationName,
			},
		})
		resultTypeCount.Increment()

		// update status code counter
		statusCodeCount := stats.Counter(&bfly.SchemaRef{
			Name: fmt.Sprintf("/rpc/%s/status", call.side),
			Keys: []string{
				"com.prztl.home.prpc",
				call.ApiName,
				call.OperationName,
				fmt.Sprintf("%d", call.StatusCode),
			},
		})
		statusCodeCount.Increment()
	*/
}
