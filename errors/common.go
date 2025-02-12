package errors

import "fmt"

func InvalidParamsErr(err error) error {
	return E(Invalid, "invalid params", err)
}

func InvalidBodyErr(err error) error {
	return E(Invalid, "invalid request body", err)
}

func ValidationFailedErr(err error) error {
	return E(Invalid, "validation failed", err)
}

func EmptyParamErr(field string) error {
	ve := ValidationErrs()
	ve.Add(field, "cannot be empty")
	return E(Invalid, "validation failed", ve.Err())
}

// ConflictErr returns a formated error for conflict check
func ConflictErr(appID, messageID string, err error) error {
	return fmt.Errorf("conflict check for %s, messageId %s failed: %s", appID, messageID, err.Error())
}
