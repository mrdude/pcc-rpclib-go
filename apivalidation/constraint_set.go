package apivalidation

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type ConstraintSet struct {
	Min, Max int // min/max length. if -1, this range bound is unset

	HasRegex bool   // does this constraint set have a regex?
	Regex    string // if HasRegex=true, the regex that must be obeyed
}

func (cs *ConstraintSet) SetMin(min int) {
	cs.SetRange(min, -1)
}

func (cs *ConstraintSet) SetMax(max int) {
	cs.SetRange(-1, max)
}

func (cs *ConstraintSet) SetRange(min, max int) {
	cs.Min = min
	cs.Max = max
}

func (cs *ConstraintSet) SetRegex(regex string) {
	cs.HasRegex = true
	cs.Regex = regex
}

func (cs *ConstraintSet) SetNoRegex() {
	cs.HasRegex = false
	cs.Regex = ""
}

func (cs *ConstraintSet) String() string {
	var elem []string

	// range
	switch {
	case cs.Min == -1 && cs.Max == -1:
		// do nothing
	case cs.Min != -1 && cs.Max == -1:
		elem = append(elem,
			fmt.Sprintf("range=[%d,)", cs.Min))
	case cs.Min == -1 && cs.Max != -1:
		elem = append(elem,
			fmt.Sprintf("range=(,%d]", cs.Max))
	case cs.Min != -1 && cs.Max != -1:
		elem = append(elem,
			fmt.Sprintf("range=[%d,%d]", cs.Min, cs.Max))
	}

	// regex
	if cs.HasRegex {
		elem = append(elem,
			fmt.Sprintf("regex=/%s/", cs.Regex))
	}

	return fmt.Sprintf("ConstraintSet{%s}", strings.Join(elem, ","))
}

// ValidateValue validates the given value
// Any error returned will be of type *Error
func (cs *ConstraintSet) ValidateValue(v *ValidationContext, obj any) error {
	switch obj := obj.(type) {
	case nil:
		return nil
	case bool:
		// nothing to do
	case int, int8, int16, int32, int64:
		val := func() int64 {
			switch obj := obj.(type) {
			case int:
				return int64(obj)
			case int8:
				return int64(obj)
			case int16:
				return int64(obj)
			case int32:
				return int64(obj)
			case int64:
				return obj
			default:
				panic(errors.New("this should never happen"))
			}
		}()
		if err := cs.validateInt(v, val); err != nil {
			return err
		}
	case uint, uint8, uint16, uint32, uint64:
		val := func() uint64 {
			switch obj := obj.(type) {
			case uint:
				return uint64(obj)
			case uint8:
				return uint64(obj)
			case uint16:
				return uint64(obj)
			case uint32:
				return uint64(obj)
			case uint64:
				return obj
			default:
				panic(errors.New("this should never happen"))
			}
		}()
		if err := cs.validateUint(v, val); err != nil {
			return err
		}
	case float32, float64:
		val := func() float64 {
			switch obj := obj.(type) {
			case float32:
				return float64(obj)
			case float64:
				return obj
			default:
				panic(errors.New("this should never happen"))
			}
		}()
		if err := cs.validateFloat(v, val); err != nil {
			return err
		}
	case string:
		if err := cs.validateString(v, obj); err != nil {
			return err
		}
	default:
		err := validate(v, obj)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cs *ConstraintSet) ValidateCollectionLength(v *ValidationContext, val int) error {
	// range
	if cs.Min != -1 && val < cs.Min {
		return v.WrapError(fmt.Errorf("length of '%d' must be in range %s", val, cs.rangeString()))
	}

	if cs.Max != -1 && val > cs.Max {
		return v.WrapError(fmt.Errorf("length of '%d' must be in range %s", val, cs.rangeString()))
	}

	return nil
}

func (cs *ConstraintSet) rangeString() string {
	rangeStr := ""

	if cs.Min == -1 {
		rangeStr += "(,"
	} else {
		rangeStr += fmt.Sprintf("[%d,", cs.Min)
	}

	if cs.Max == -1 {
		rangeStr += ")"
	} else {
		rangeStr += fmt.Sprintf("%d]", cs.Max)
	}

	return rangeStr
}

func (cs *ConstraintSet) validateInt(v *ValidationContext, val int64) error {
	// range
	if cs.Min != -1 && val < int64(cs.Min) {
		return v.WrapError(fmt.Errorf("value %d must be in range %s", val, cs.rangeString()))
	}

	if cs.Max != -1 && val > int64(cs.Max) {
		return v.WrapError(fmt.Errorf("value %d must be in range %s", val, cs.rangeString()))
	}

	return nil
}

func (cs *ConstraintSet) validateUint(v *ValidationContext, val uint64) error {
	// range
	if cs.Min != -1 && val < uint64(cs.Min) {
		return v.WrapError(fmt.Errorf("value %d must be in range %s", val, cs.rangeString()))
	}

	if cs.Max != -1 && val > uint64(cs.Max) {
		return v.WrapError(fmt.Errorf("value %d must be in range %s", val, cs.rangeString()))
	}

	return nil
}

func (cs *ConstraintSet) validateFloat(v *ValidationContext, val float64) error {
	// range
	// FIXME allow fractional values here
	if cs.Min != -1 && val < float64(cs.Min) {
		return v.WrapError(fmt.Errorf("value %f must be in range %s", val, cs.rangeString()))
	}

	if cs.Max != -1 && val > float64(cs.Max) {
		return v.WrapError(fmt.Errorf("value %f must be in range %s", val, cs.rangeString()))
	}

	return nil
}

func (cs *ConstraintSet) validateString(v *ValidationContext, val string) error {
	// range
	L := len(val)

	if cs.Min != -1 && L < cs.Min {
		return v.WrapError(fmt.Errorf("length of '%s' must be in range %s", val, cs.rangeString()))
	}

	if cs.Max != -1 && L > cs.Max {
		return v.WrapError(fmt.Errorf("length of '%s' must be in range %s", val, cs.rangeString()))
	}

	// regex
	if cs.HasRegex {
		re, err := regexp.Compile(cs.Regex) // TODO cache this
		if err != nil {
			// this should never happen, but we still need to handle it anyway
			return fmt.Errorf("constraint set regex error: %s: %w", cs.Regex, err)
		}

		if !re.MatchString(val) {
			return v.WrapError(fmt.Errorf("value '%s' must match /%s/",
				val, re.String()))
		}
	}

	return nil
}
