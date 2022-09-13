package shoptree

import "errors"

var (
	ErrReferenceIDIsRequired      = errors.New("reference id is required")
	ErrReferenceTypeIsRequired    = errors.New("reference type is required")
	ErrLocationIDIsRequired       = errors.New("location id is required")
	ErrProductVariantIDIsRequired = errors.New("product variant id is required")
	ErrInStockIsRequired          = errors.New("in stock is required")
	ErrQuantityChangedIsRequired  = errors.New("quantity changed is required")
	ErrInvalidInStock             = errors.New("invalid in stock value")
	ErrInvalidReferenceType       = errors.New("invalid reference type")
	ErrEnabledIsRequired          = errors.New("enabled is required")

	ErrContenTypeIsRequired    = errors.New("content type is required")
	ErrInvalidContentType      = errors.New("content type should be application/json")
	ErrXClientAPIKeyIsRequired = errors.New("x client api key is required")
	ErrInvalidXClientAPIKey    = errors.New("invalid x client api key")

	// ErrAuthKeyNotFound happens when no auth key is passed when initializing a new handler.
	ErrAuthKeyNotFound = errors.New("auth key not found")

	ErrMarshallingUnsuccessful     = errors.New("marshalling unsuccessful")
	ErrWriteToResponseUnsuccessful = errors.New("write to response unsuccessful")
)
