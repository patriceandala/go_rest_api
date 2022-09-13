package shoptree

import (
	"encoding/json"
	"math"
	"net/http"

	"github.com/rs/zerolog"

	// protobuf
	inpb "github.com/dropezy/proto/v1/inventory"
	prpb "github.com/dropezy/proto/v1/product"
)

// reference types listed by shoptree
const (
	reference_type_internal_order           = "internal_order"
	reference_type_purchase_order           = "purchase_order"
	reference_type_transfer_order           = "transfer_order"
	reference_type_stock_take               = "stock_take"
	reference_type_stock_adjustment         = "stock_adjustment"
	reference_type_preparation              = "preparation"
	reference_type_separation               = "separation"
	reference_type_order                    = "order"
	reference_type_order_modifier           = "order_modifier"
	reference_type_order_composite          = "order_composite"
	reference_type_order_modifier_composite = "order_modifier_composite"
)

type UpdateStockRequest struct {
	ReferenceID      string   `json:"reference_id"`
	ReferenceType    string   `json:"reference_type"`
	LocationID       string   `json:"location_id"`
	ProductVariantID string   `json:"product_variant_id"`
	InStock          *float64 `json:"in_stock"`
	QuantityChanged  *float64 `json:"quantity_changed"`
}

type UpdateProductStatusRequest struct {
	LocationID       string `json:"location_id"`
	ProductVariantID string `json:"product_variant_id"`
	Enabled          *bool  `json:"enabled"`
}

// Validate checks all UpdateStockRequest parameters, return error if empty.
func (u *UpdateStockRequest) Validate() error {
	// check if any parameter is empty
	switch {
	case u.ReferenceID == "":
		return ErrReferenceIDIsRequired
	case u.ReferenceType == "":
		return ErrReferenceTypeIsRequired
	case u.LocationID == "":
		return ErrLocationIDIsRequired
	case u.ProductVariantID == "":
		return ErrProductVariantIDIsRequired
	case u.InStock == nil:
		return ErrInStockIsRequired
	case u.QuantityChanged == nil:
		return ErrQuantityChangedIsRequired
	}

	return nil
}

// ToPB converts UpdateStockRequest to proto format
func (u *UpdateStockRequest) ToPB() (*inpb.UpdateStockRequest, error) {
	// check if InStock has decimal points
	if math.Mod(*u.InStock, 1) != 0 {
		return nil, ErrInvalidInStock
	}

	return &inpb.UpdateStockRequest{
		StoreId:          u.LocationID,
		ProductVariantId: u.ProductVariantID,
		Quantity:         int32(*u.InStock),
		Source:           inpb.UpdateSource_UPDATE_SOURCE_EXTERNAL,
	}, nil
}

// Validate checks all UpdateProductStatusRequest parameters, return error if empty.
func (u *UpdateProductStatusRequest) Validate() error {
	// check if any parameter is empty
	switch {
	case u.LocationID == "":
		return ErrLocationIDIsRequired
	case u.ProductVariantID == "":
		return ErrProductVariantIDIsRequired
	case u.Enabled == nil:
		return ErrEnabledIsRequired
	}

	return nil
}

// ToPB converts UpdateProductStatus to proto format
func (u *UpdateProductStatusRequest) ToPB() *inpb.UpdateStatusRequest {
	var status prpb.ProductStatus
	if *u.Enabled {
		status = prpb.ProductStatus_PRODUCT_STATUS_ENABLED
	} else {
		status = prpb.ProductStatus_PRODUCT_STATUS_DISABLED
	}

	return &inpb.UpdateStatusRequest{
		StoreId:          u.LocationID,
		ProductVariantId: u.ProductVariantID,
		Status:           status,
		Source:           inpb.UpdateSource_UPDATE_SOURCE_EXTERNAL,
	}
}

type Response struct {
	Message string `json:"message"`
}

// responseJSON create mashaled response and return response.
func responseJSON(logger zerolog.Logger, w http.ResponseWriter, code int, message string) {
	logger = logger.With().Str("method", "responseJSON").Logger()

	w.Header().Set("Content-Type", "application/json")

	res, err := json.Marshal(&Response{Message: message})
	if err != nil {
		logger.Err(ErrMarshallingUnsuccessful).Msg(ErrMarshallingUnsuccessful.Error())
	}

	w.WriteHeader(code)

	if _, err = w.Write(res); err != nil {
		logger.Err(ErrWriteToResponseUnsuccessful).Msg(ErrWriteToResponseUnsuccessful.Error())
	}
}

// validateHeaders to check if Content-Type and X-Client-Api-Key is given and not empty.
func validateHeaders(logger zerolog.Logger, h http.Header, authKey string) error {
	// check content type, expect application/json
	ct := h.Get("Content-Type")
	if ct == "" {
		logger.Err(ErrContenTypeIsRequired).Msg(ErrContenTypeIsRequired.Error())
		return ErrContenTypeIsRequired
	}
	if ct != "application/json" {
		logger.Err(ErrInvalidContentType).Msg(ErrInvalidContentType.Error())
		return ErrInvalidContentType
	}

	// check x client api key
	apiKey := h.Get("X-Client-Api-Key")
	if apiKey == "" {
		logger.Err(ErrXClientAPIKeyIsRequired).Msg(ErrXClientAPIKeyIsRequired.Error())
		return ErrXClientAPIKeyIsRequired
	}
	if validApiKey := authKey; apiKey != validApiKey {
		logger.Err(ErrInvalidXClientAPIKey).Msg(ErrInvalidXClientAPIKey.Error())
		return ErrInvalidXClientAPIKey
	}

	return nil
}
