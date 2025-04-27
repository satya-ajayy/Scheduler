package errors

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
