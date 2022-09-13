package shoptree

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"

	// protobuf

	inpbmock "github.com/dropezy/proto/mock/inventory"
	inpb "github.com/dropezy/proto/v1/inventory"
)

var validAuthKey = "valid-x-client-api-key"

func newTestHandler(client *inpbmock.MockInventoryServiceClient) *Handler {
	h, err := NewHandler(validAuthKey, client)
	if err != nil {
		log.Fatal(err)
	}
	return h
}

func TestNewHandler(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockClient := inpbmock.NewMockInventoryServiceClient(ctrl)

	t.Run("Success", func(t *testing.T) {
		t.Parallel()

		if _, err := NewHandler(validAuthKey, mockClient); err != nil {
			t.Fatalf("NewHandler(), got = %v, want = %v", err, nil)
		}
	})

	t.Run("EmptyAuthKey", func(t *testing.T) {
		t.Parallel()

		if _, err := NewHandler("", mockClient); err != ErrAuthKeyNotFound {
			t.Fatalf("NewHandler(), got = %v, want %v", err, ErrAuthKeyNotFound)
		}
	})
}

func TestHandleStockUpdate(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockClient := inpbmock.NewMockInventoryServiceClient(ctrl)

	const (
		validContentType = "application/json"
		validRequest     = `[{
			"reference_id": "valid-reference-id",
			"reference_type": "stock_adjustment",
			"location_id": "valid-location-id",
			"product_variant_id": "valid-product-variant-id",
			"in_stock": 1,
			"quantity_changed": -1
		}]`
	)
	validHeaders := map[string]string{
		"Content-Type":     validContentType,
		"X-Client-API-Key": validAuthKey,
	}

	t.Run("Success", func(t *testing.T) {
		// mock success response
		mockClient.
			EXPECT().
			UpdateStock(
				gomock.Any(), // context
				gomock.Any(), // update stock request
			).
			Return(&inpb.UpdateStockResponse{}, nil)

		h := newTestHandler(mockClient)
		w := httptest.NewRecorder()

		updateStockRequest := []byte(validRequest)

		r, err := http.NewRequest(http.MethodPost, "/shoptree/stock-update", bytes.NewBuffer(updateStockRequest))
		if err != nil {
			t.Fatal(err)
		}

		r.Header.Set("X-Client-Api-Key", validAuthKey)
		r.Header.Set("Content-Type", validContentType)

		handler := http.HandlerFunc(h.HandleStockUpdate)
		handler.ServeHTTP(w, r)

		resp := w.Result()
		if gotStatusCode := resp.StatusCode; gotStatusCode != http.StatusOK {
			t.Fatalf("HandleStockUpdate(), got = %v, want = %v", gotStatusCode, http.StatusOK)
		}

		if gotContentType := resp.Header.Get("Content-Type"); gotContentType != validContentType {
			t.Fatalf("HandleStockUpdate(), got = %v, want = %v", gotContentType, validContentType)
		}
	})

	// failed tests scenario
	tests := []struct {
		name              string
		method            string
		headers           map[string]string
		in                []byte
		wantErr           string
		wantErrCode       int
		inventoryClientFn []func(m *inpbmock.MockInventoryServiceClient)
	}{
		{
			name:        "InvalidMethod",
			method:      http.MethodGet,
			headers:     validHeaders,
			in:          []byte(validRequest),
			wantErr:     "expecting http method post, got: GET",
			wantErrCode: http.StatusMethodNotAllowed,
		},
		{
			name:    "EmptyReferenceID",
			method:  http.MethodPost,
			headers: validHeaders,
			in: []byte(`[{
				"reference_type": "stock_adjustment",
				"location_id": "valid-location-id",
				"product_variant_id": "valid-product-variant-id",
				"in_stock": 1,
				"quantity_changed": -1
			}]`),
			wantErr:     ErrReferenceIDIsRequired.Error(),
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:    "EmptyReferenceType",
			method:  http.MethodPost,
			headers: validHeaders,
			in: []byte(`[{
				"reference_id": "valid-reference-id",
				"location_id": "valid-location-id",
				"product_variant_id": "valid-product-variant-id",
				"in_stock": 1,
				"quantity_changed": -1
			}]`),
			wantErr:     ErrReferenceTypeIsRequired.Error(),
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:    "EmptyLocationID",
			method:  http.MethodPost,
			headers: validHeaders,
			in: []byte(`[{
				"reference_id": "valid-reference-id",
				"reference_type": "stock_adjustment",
				"product_variant_id": "valid-product-variant-id",
				"in_stock": 1,
				"quantity_changed": -1
			}]`),
			wantErr:     ErrLocationIDIsRequired.Error(),
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:    "EmptyProductVariantID",
			method:  http.MethodPost,
			headers: validHeaders,
			in: []byte(`[{
				"reference_id": "valid-reference-id",
				"reference_type": "stock_adjustment",
				"location_id": "valid-location-id",
				"in_stock": 1,
				"quantity_changed": -1
			}]`),
			wantErr:     ErrProductVariantIDIsRequired.Error(),
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:    "EmptyInStock",
			method:  http.MethodPost,
			headers: validHeaders,
			in: []byte(`[{
				"reference_id": "valid-reference-id",
				"reference_type": "stock_adjustment",
				"location_id": "valid-location-id",
				"product_variant_id": "valid-product-variant-id",
				"quantity_changed": -1
			}]`),
			wantErr:     ErrInStockIsRequired.Error(),
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:    "EmptyQuantityChanged",
			method:  http.MethodPost,
			headers: validHeaders,
			in: []byte(`[{
				"reference_id": "valid-reference-id",
				"reference_type": "stock_adjustment",
				"location_id": "valid-location-id",
				"product_variant_id": "valid-product-variant-id",
				"in_stock": 1
			}]`),
			wantErr:     ErrQuantityChangedIsRequired.Error(),
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:    "InvalidInStock",
			method:  http.MethodPost,
			headers: validHeaders,
			in: []byte(`[{
				"reference_id": "valid-reference-id",
				"reference_type": "stock_adjustment",
				"location_id": "valid-location-id",
				"product_variant_id": "valid-product-variant-id",
				"in_stock": 1.23,
				"quantity_changed": -1
			}]`),
			wantErr:     ErrInvalidInStock.Error(),
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:    "InvalidReferenceType",
			method:  http.MethodPost,
			headers: validHeaders,
			in: []byte(`[{
				"reference_id": "valid-reference-id",
				"reference_type": "invalid-reference-type",
				"location_id": "valid-location-id",
				"product_variant_id": "valid-product-variant-id",
				"in_stock": 1,
				"quantity_changed": -1
			}]`),
			wantErr:     ErrInvalidReferenceType.Error(),
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:        "EmptyRequestInput",
			method:      http.MethodPost,
			headers:     validHeaders,
			wantErr:     "invalid request data",
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:   "EmptyContentTypeHeader",
			method: http.MethodPost,
			headers: map[string]string{
				"X-Client-Api-Key": validAuthKey,
			},
			in:          []byte(validRequest),
			wantErr:     ErrContenTypeIsRequired.Error(),
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:   "InvalidContentTypeHeader",
			method: http.MethodPost,
			headers: map[string]string{
				"Content-Type":     "multipart/form-data",
				"X-Client-Api-Key": validAuthKey,
			},
			in:          []byte(validRequest),
			wantErr:     ErrInvalidContentType.Error(),
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:   "EmptyAPIKeyHeader",
			method: http.MethodPost,
			headers: map[string]string{
				"Content-Type": validContentType,
			},
			in:          []byte(validRequest),
			wantErr:     ErrXClientAPIKeyIsRequired.Error(),
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:   "InvalidAuthKeyHeader",
			method: http.MethodPost,
			headers: map[string]string{
				"Content-Type":     validContentType,
				"X-Client-Api-Key": "invalid-client-api-key",
			},
			in:          []byte(validRequest),
			wantErr:     ErrInvalidXClientAPIKey.Error(),
			wantErrCode: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			h := newTestHandler(mockClient)
			w := httptest.NewRecorder()

			r, err := http.NewRequest(test.method, "/shoptree/stock-update", bytes.NewBuffer(test.in))
			if err != nil {
				t.Fatal(err)
			}

			for k, v := range test.headers {
				r.Header.Set(k, v)
			}

			handler := http.HandlerFunc(h.HandleStockUpdate)
			handler.ServeHTTP(w, r)

			resp := w.Result()

			if gotStatusCode := resp.StatusCode; gotStatusCode != test.wantErrCode {
				t.Fatalf("HandleStockUpdate(), got = %v, want = %v", gotStatusCode, test.wantErrCode)
			}

			if gotContentType := resp.Header.Get("Content-Type"); gotContentType != validContentType {
				t.Fatalf("HandleStockUpdate(), got = %v, want =%v", gotContentType, validContentType)
			}

			// check response body
			req := &struct {
				Message string `json:"message"`
			}{}
			if err := json.NewDecoder(resp.Body).Decode(req); err != nil {
				t.Fatal("Failed to decode response")
			}
			if gotMesssage := req.Message; gotMesssage != test.wantErr {
				t.Fatalf("HandleStockUpdate(), got = %v, want = %v", gotMesssage, test.wantErr)
			}
		})
	}
}

func TestHandleProductStatusUpdate(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockClient := inpbmock.NewMockInventoryServiceClient(ctrl)

	const (
		validContentType = "application/json"
		validRequest     = `[{
			"location_id": "valid-location-id",
			"product_variant_id": "valid-product-variant-id",
			"enabled": false
		}]`
	)
	validHeaders := map[string]string{
		"Content-Type":     validContentType,
		"X-Client-API-Key": validAuthKey,
	}

	t.Run("Success", func(t *testing.T) {
		// mock success response
		mockClient.
			EXPECT().
			UpdateStatus(
				gomock.Any(), // context
				gomock.Any(), // update status request
			).
			Return(&inpb.UpdateStatusResponse{}, nil)

		h := newTestHandler(mockClient)
		w := httptest.NewRecorder()

		updateStatusRequest := []byte(validRequest)

		r, err := http.NewRequest(http.MethodPost, "/shoptree/product-status-update", bytes.NewBuffer(updateStatusRequest))
		if err != nil {
			t.Fatal(err)
		}

		r.Header.Set("X-Client-Api-Key", validAuthKey)
		r.Header.Set("Content-Type", validContentType)

		handler := http.HandlerFunc(h.HandleProductStatusUpdate)
		handler.ServeHTTP(w, r)

		resp := w.Result()
		if gotStatusCode := resp.StatusCode; gotStatusCode != http.StatusOK {
			t.Fatalf("HandleProductStatusUpdate() error, got = %v, want = %v", gotStatusCode, http.StatusOK)
		}

		if gotContentType := resp.Header.Get("Content-Type"); gotContentType != validContentType {
			t.Fatalf("HandleProductStatusUpdate() error, got = %v, want = %v", gotContentType, validContentType)
		}
	})

	tests := []struct {
		name        string
		method      string
		headers     map[string]string
		in          []byte
		wantErr     string
		wantErrCode int
	}{
		{
			name:        "InvalidMethod",
			method:      http.MethodGet,
			headers:     validHeaders,
			in:          []byte(validRequest),
			wantErr:     "expecting http method post, got: GET",
			wantErrCode: http.StatusMethodNotAllowed,
		},
		{
			name:    "EmptyLocationID",
			method:  http.MethodPost,
			headers: validHeaders,
			in: []byte(`[{
				"product_variant_id": "valid-product-variant-id",
				"enabled": true
			}]`),
			wantErr:     ErrLocationIDIsRequired.Error(),
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:    "EmptyProductVariantID",
			method:  http.MethodPost,
			headers: validHeaders,
			in: []byte(`[{
				"location_id": "valid-location-id",
				"enabled": true
			}]`),
			wantErr:     ErrProductVariantIDIsRequired.Error(),
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:    "EmptyEnabled",
			method:  http.MethodPost,
			headers: validHeaders,
			in: []byte(`[{
				"location_id": "valid-location-id",
				"product_variant_id": "valid-product-variant-id"
			}]`),
			wantErr:     ErrEnabledIsRequired.Error(),
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:        "EmptyRequestInput",
			method:      http.MethodPost,
			headers:     validHeaders,
			wantErr:     "invalid request data",
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:   "EmptyContentTypeHeader",
			method: http.MethodPost,
			headers: map[string]string{
				"X-Client-Api-Key": validAuthKey,
			},
			in:          []byte(validRequest),
			wantErr:     ErrContenTypeIsRequired.Error(),
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:   "InvalidContentTypeHeader",
			method: http.MethodPost,
			headers: map[string]string{
				"Content-Type":     "multipart/form-data",
				"X-Client-Api-Key": validAuthKey,
			},
			in:          []byte(validRequest),
			wantErr:     ErrInvalidContentType.Error(),
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:   "EmptyAPIKeyHeader",
			method: http.MethodPost,
			headers: map[string]string{
				"Content-Type": validContentType,
			},
			in:          []byte(validRequest),
			wantErr:     ErrXClientAPIKeyIsRequired.Error(),
			wantErrCode: http.StatusBadRequest,
		},
		{
			name:   "InvalidAuthKeyHeader",
			method: http.MethodPost,
			headers: map[string]string{
				"Content-Type":     validContentType,
				"X-Client-Api-Key": "invalid-client-api-key",
			},
			in:          []byte(validRequest),
			wantErr:     ErrInvalidXClientAPIKey.Error(),
			wantErrCode: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			h := newTestHandler(mockClient)
			w := httptest.NewRecorder()

			r, err := http.NewRequest(test.method, "/shoptree/product-status-update", bytes.NewBuffer(test.in))
			if err != nil {
				t.Fatal(err)
			}

			for k, v := range test.headers {
				r.Header.Set(k, v)
			}

			handler := http.HandlerFunc(h.HandleProductStatusUpdate)
			handler.ServeHTTP(w, r)

			resp := w.Result()

			if gotStatusCode := resp.StatusCode; gotStatusCode != test.wantErrCode {
				t.Fatalf("HandleProductStatusUpdate(), got = %v, want = %v", gotStatusCode, test.wantErrCode)
			}

			if gotContentType := resp.Header.Get("Content-Type"); gotContentType != validContentType {
				t.Fatalf("HandleProductStatusUpdate(), got = %v, want =%v", gotContentType, validContentType)
			}

			// check response body
			req := &struct {
				Message string `json:"message"`
			}{}
			if err := json.NewDecoder(resp.Body).Decode(req); err != nil {
				t.Fatal("Failed to decode response")
			}
			if gotMessage := req.Message; gotMessage != test.wantErr {
				t.Fatalf("HandleProductStatusUpdate(), got = %v, want = %v", gotMessage, test.wantErr)
			}
		})
	}
}
