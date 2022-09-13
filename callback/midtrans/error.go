package midtrans

import "errors"

var (
	ErrContenTypeIsRequired = errors.New("content type is required")
	ErrInvalidContentType   = errors.New("content type should be application/json")
	ErrInvalidSignature     = errors.New("invalid signature")
	ErrInvalidStatusCode    = errors.New("invalid status code")

	ErrMarshallingUnsuccessful     = errors.New("marshalling unsuccessful")
	ErrWriteToResponseUnsuccessful = errors.New("write to response unsuccessful")
	ErrUnsupportedPaymentMethod    = errors.New("unsupported payment method")

	ErrInternalServerError = errors.New("internal server error")
)
