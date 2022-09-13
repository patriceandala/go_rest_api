package shoptree

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/rs/zerolog"

	// protobuf
	"github.com/dropezy/internal/logging"
	inpb "github.com/dropezy/proto/v1/inventory"
)

const handlerName = "shoptree"

// Handler is a http handler to receive callbacks from shoptree
// and forward it to our internal gRPC services.
type Handler struct {
	authKey string
	client  inpb.InventoryServiceClient
}

// NewHandler returns a new inventory handler.
func NewHandler(authKey string, client inpb.InventoryServiceClient) (*Handler, error) {
	switch "" {
	case authKey:
		return nil, ErrAuthKeyNotFound
	}
	return &Handler{
		authKey: authKey,
		client:  client,
	}, nil
}

// HandleStockUpdate handles callback from Shoptree to update
// product stock in a specific location.
func (h *Handler) HandleStockUpdate(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context()).With().Str("handler", handlerName).Logger()

	if r.Method != http.MethodPost {
		err := fmt.Errorf("expecting http method post, got: %s", r.Method)
		logger.Err(err).Send()

		responseJSON(logger, w,
			http.StatusMethodNotAllowed,
			err.Error(),
		)
		return
	}

	if err := validateHeaders(logger, r.Header, h.authKey); err != nil {
		responseJSON(logger, w, http.StatusBadRequest,
			err.Error(),
		)
		return
	}

	var data []*UpdateStockRequest
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		logger.Err(err).Msg("failed to decode request data")

		if logger.GetLevel() == zerolog.DebugLevel {
			if buf, err := httputil.DumpRequest(r, true); err == nil {
				fmt.Println("---[ PRODUCT STOCK UPDATE REQUEST DUMP ]-------------")
				fmt.Printf("%s\n", string(buf))
				fmt.Println("-----------------------------------------------------")
			}
		}

		responseJSON(logger, w, http.StatusBadRequest,
			"invalid request data",
		)
		return
	}

	for _, req := range data {
		// add product variant id and location id to logger
		logger := logger.With().Fields(map[string]interface{}{
			"shoptree_variant_id":  req.ProductVariantID,
			"shoptree_location_id": req.LocationID,
		}).Logger()

		// check if the request contains all required fields
		if err := req.Validate(); err != nil {
			logger.Err(err).Send()

			responseJSON(logger, w, http.StatusBadRequest,
				err.Error(),
			)
			return
		}

		// only update stock to inventory service if the reference type is not type "order",
		// and return error if the reference type is not listed.
		switch req.ReferenceType {
		case reference_type_order,
			reference_type_internal_order,
			reference_type_purchase_order,
			reference_type_transfer_order,
			reference_type_stock_take,
			reference_type_stock_adjustment,
			reference_type_preparation,
			reference_type_separation,
			reference_type_order_modifier,
			reference_type_order_composite,
			reference_type_order_modifier_composite:
			inventory, err := req.ToPB()
			if err != nil {
				if errors.Is(ErrInvalidInStock, err) {
					logger.Err(err).Send()

					responseJSON(logger, w, http.StatusBadRequest,
						err.Error(),
					)
					return
				}
				logger.Debug().Msg("failed to convert update stock request to pb")

				responseJSON(logger, w, http.StatusInternalServerError,
					"failed to update stock",
				)
				return
			}
			// request update stock to inventory service.
			if _, err := h.client.UpdateStock(r.Context(), inventory); err != nil {
				logger.Err(err).Msg("failed to update stock to inventory service")

				responseJSON(logger, w, http.StatusInternalServerError,
					"failed to update stock",
				)
				return
			}

			// logs the returned SKU and Location ID
			logger.Info().Msg("successfully update stock to inventory service")
		default:
			logger.
				Err(ErrInvalidReferenceType).
				Str("reference_type", req.ReferenceType).
				Msg(ErrInvalidReferenceType.Error())

			responseJSON(logger, w, http.StatusBadRequest,
				ErrInvalidReferenceType.Error(),
			)
			return
		}

	}

	logger.Info().Msg("successfully processing update stock request")
	responseJSON(logger, w, http.StatusOK,
		"success",
	)
}

func (h *Handler) HandleProductStatusUpdate(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context()).With().Str("handler", handlerName).Logger()

	if r.Method != http.MethodPost {
		err := fmt.Errorf("expecting http method post, got: %s", r.Method)
		logger.Err(err).Send()

		responseJSON(logger, w,
			http.StatusMethodNotAllowed,
			err.Error(),
		)
		return
	}

	if err := validateHeaders(logger, r.Header, h.authKey); err != nil {
		responseJSON(logger, w, http.StatusBadRequest,
			err.Error(),
		)
		return
	}

	var data []*UpdateProductStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		logger.Err(err).Msg("failed to decode request data")

		if logger.GetLevel() == zerolog.DebugLevel {
			if buf, err := httputil.DumpRequest(r, true); err == nil {
				fmt.Println("---[ PRODUCT STATUS UPDATE REQUEST DUMP ]------------")
				fmt.Printf("%s\n", string(buf))
				fmt.Println("-----------------------------------------------------")
			}
		}

		responseJSON(logger, w, http.StatusBadRequest,
			"invalid request data",
		)
		return
	}

	for _, req := range data {
		// add product variant id and location id to logger
		logger := logger.With().Fields(map[string]interface{}{
			"shoptree_variant_id":  req.ProductVariantID,
			"shoptree_location_id": req.LocationID,
		}).Logger()

		// check if the request contains all required fields
		if err := req.Validate(); err != nil {
			logger.Err(err).Send()

			responseJSON(logger, w, http.StatusBadRequest,
				err.Error(),
			)
			return
		}

		inventory := req.ToPB()
		// request update product variant status to inventory service.
		if _, err := h.client.UpdateStatus(r.Context(), inventory); err != nil {
			logger.Err(err).Msg("failed to update status to inventory service")

			responseJSON(logger, w, http.StatusInternalServerError,
				"failed to update product variant status",
			)
			return
		}

	}

	logger.Info().Msg("successfully processing update product status request")
	responseJSON(logger, w, http.StatusOK,
		"success",
	)
}
