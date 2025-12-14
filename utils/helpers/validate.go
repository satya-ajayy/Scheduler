package helpers

import (
	// Local Packages
	errors "scheduler/errors"
)

func ValidateRequiredString(ve *errors.ValidationErrorBuilder, field, value string) {
	if value == "" {
		ve.Add(field, "cannot be empty")
	}
}

func ValidateRequiredInt(ve *errors.ValidationErrorBuilder, field string, value int) {
	if value <= 0 {
		ve.Add(field, "need to be greater than zero")
	}
}
