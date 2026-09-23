package apirt

import (
	"encoding/json"
	"fmt"
	"time"
)

var _ json.Marshaler = &Time{}
var _ json.Unmarshaler = &Time{}

type Time struct {
	time.Time
}

const timestampFormat = "2006-01-02T15:04:05Z"

func (t *Time) MarshalJSON() ([]byte, error) {
	str := t.UTC().Format(timestampFormat)
	return []byte(fmt.Sprintf("\"%s\"", str)), nil
}

func (t *Time) UnmarshalJSON(js []byte) (err error) {
	var str string
	err = json.Unmarshal(js, &str)
	if err != nil {
		return
	}

	parsedTime, err := time.Parse(timestampFormat, str)
	if err != nil {
		return err
	}

	t.Time = parsedTime
	return nil
}
