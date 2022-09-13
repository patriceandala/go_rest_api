package mileapp

import "errors"

var (
	ErrTaskRefIDIsRequired         = errors.New("taskRefId is required")
	ErrStatusIsRequired            = errors.New("taskStatus is required")
	ErrOrderNumberIsRequired       = errors.New("order number is required")
	ErrInvalidStatus               = errors.New("taskStatus is invalid")
	ErrContenTypeIsRequired        = errors.New("content-type is required")
	ErrInvalidContentType          = errors.New("invalid content-type")
	ErrXAPIKeyIsRequired           = errors.New("x-api-key is required")
	ErrInvalidXAPIKey              = errors.New("invalid x-api-key")
	ErrMarshallingUnsuccessful     = errors.New("marshalling unsuccessful")
	ErrWriteToResponseUnsuccessful = errors.New("write to response unsuccessful")
)
