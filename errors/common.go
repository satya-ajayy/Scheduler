package errors

func InvalidBodyErr(err error) error {
	return NewError(Invalid, "invalid request body", err)
}

func ValidationFailedErr(err error) error {
	return NewError(Invalid, "validation failed", err)
}

func EmptyParamErr(field string) error {
	ve := ValidationErrs()
	ve.Add(field, "cannot be empty")
	return NewError(Invalid, "validation failed", ve.Err())
}
