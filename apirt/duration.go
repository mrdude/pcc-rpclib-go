package apirt

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

var _ json.Marshaler = &Duration{}
var _ json.Unmarshaler = &Duration{}

type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	durationMap := map[string]string{
		"nanos":   strconv.FormatInt(d.Duration.Nanoseconds(), 10),
		"seconds": strconv.FormatInt(int64(d.Duration.Seconds()), 10),
	}

	return json.Marshal(durationMap)
}

func (d *Duration) UnmarshalJSON(js []byte) (err error) {
	var durationMap map[string]string
	err = json.Unmarshal(js, &durationMap)
	if err != nil {
		return
	}

	if nanosStr, hasNanos := durationMap["nanos"]; hasNanos {
		nanos, err := strconv.ParseInt(nanosStr, 10, 64)
		if err != nil {
			return err
		}

		d.Duration = time.Duration(nanos) * time.Nanosecond
		return nil
	} else {
		return errors.New("invalid duration")
	}
}
