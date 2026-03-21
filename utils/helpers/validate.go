package helpers

import (
	// Go Internal Packages
	"log"
	"time"

	// Local Packages
	errors "scheduler/errors"
)

func ValidateRequiredString(ve *errors.ValidationErrorBuilder, field, value string) {
	if value == "" {
		ve.Add(field, "cannot be empty")
	}
}

func ValidateRequiredSlice[T any](ve *errors.ValidationErrorBuilder, field string, value []T) {
	if len(value) == 0 {
		ve.Add(field, "cannot be empty")
	}
}

type Number interface {
	~int | ~float32 | ~float64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

func ValidateRequiredNumber[T Number](ve *errors.ValidationErrorBuilder, field string, value T) {
	if value <= 0 {
		ve.Add(field, "need to be greater than zero")
	}
}

func ValidateDate(ve *errors.ValidationErrorBuilder, field, value string) {
	if value == "" {
		ve.Add(field, "cannot be empty")
		return
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		ve.Add(field, "invalid date format, expected YYYY-MM-DD")
	}
}

func LogValidationErrors(err error) {
	var ve errors.ValidationErrors
	if errors.As(err, &ve) {
		for i, fe := range ve {
			log.Printf("%d) %s: %s", i+1, fe.Field, fe.Error)
		}
	}
}
