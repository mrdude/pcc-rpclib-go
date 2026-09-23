package zipkin

import (
	"encoding/json"
	"time"
)

type Duration struct {
	time.Duration // in microseconds
}

func (d *Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.Microseconds())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var micros int64
	if err := json.Unmarshal(b, &micros); err != nil {
		return err
	}

	microDuration := time.Duration(micros) * time.Microsecond
	*d = Duration{Duration: microDuration}
	return nil
}
