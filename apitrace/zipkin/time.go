package zipkin

import (
	"encoding/json"
	"time"
)

type Time struct {
	time.Time //in epoch microseconds
}

func (t *Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Time.UnixMicro())
}

func (t *Time) UnmarshalJSON(b []byte) error {
	var micros int64
	if err := json.Unmarshal(b, &micros); err != nil {
		return err
	}

	*t = Time{Time: time.UnixMicro(micros)}
	return nil
}
