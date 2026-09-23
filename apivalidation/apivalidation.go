// Package apivalidation implements helper routines for server-side API structure validation
package apivalidation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// ValidateObject validates the given object
// Any error returned will be of type *Error
func ValidateObject(obj any) error {
	v := newValidationContext()
	err := validate(v, obj)
	if err != nil && !errors.Is(err, &Error{}) {
		err = &Error{path: "$", err: err}
	}
	return err
}

type ValidatableObject interface {
	Validate(v *ValidationContext) error
}

func validate(v *ValidationContext, obj any) error {
	// if this object implements ValidatableObject, use that to perform validation
	if valObj, ok := obj.(ValidatableObject); ok {
		err := valObj.Validate(v)
		err = v.WrapError(err)
		return err
	}

	// fall back to reflection
	err := validateObjectWithReflection(v, obj)
	if err != nil {
		return err
	}
	return nil
}

func validateObjectWithReflection(v *ValidationContext, obj any) error {
	typ := reflect.TypeOf(obj)
	val := reflect.ValueOf(obj)

	if typ.Kind() == reflect.Pointer {
		if val.IsZero() {
			return nil
		}

		return validate(v, val.Elem().Interface())
	}

	switch typ.Kind() {
	case reflect.Slice:
		L := val.Len()
		for i := 0; i < L; i++ {
			err := validate(v.ArrayIndex(i), val.Index(i).Interface())
			if err != nil {
				return err
			}
		}
	case reflect.Struct:
		L := val.NumField()
		for i := 0; i < L; i++ {
			if !typ.Field(i).IsExported() {
				continue
			}

			fieldName := structFieldName(typ.Field(i))
			err := validate(v.Field(fieldName), val.Field(i).Interface())
			if err != nil {
				return err
			}
		}
	default:
		// TODO print message?
	}

	return nil
}

func structFieldName(f reflect.StructField) string {
	js, ok := f.Tag.Lookup("json")
	if !ok {
		return f.Name
	}

	js, _, _ = strings.Cut(js, ",")
	return js
}

type ValidationContext struct {
	path string
}

func newValidationContext() *ValidationContext {
	return &ValidationContext{
		path: "$",
	}
}

func (v *ValidationContext) child(subpath string) *ValidationContext {
	return &ValidationContext{path: v.path + subpath}
}

func (v *ValidationContext) Field(name string) *ValidationContext {
	return v.child("." + name)
}

func (v *ValidationContext) ArrayIndex(index int) *ValidationContext {
	return v.child(fmt.Sprintf("[%d]", index))
}

func (v *ValidationContext) WrapError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, &Error{}) {
		return err
	}

	return &Error{path: v.path, err: err}
}

// Error represents a validation error
type Error struct {
	path string
	err  error
}

func (err *Error) Error() string {
	return fmt.Sprintf("validation error @ %s: %s", err.path, err.err)
}

func (err *Error) Unwrap() error {
	return err.err
}

func (err *Error) Is(target error) bool {
	_, ok := target.(*Error)
	return ok
}
