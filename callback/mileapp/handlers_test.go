package mileapp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/dropezy/internal/logging"
	tpbmock "github.com/dropezy/proto/mock/task"
	tpb "github.com/dropezy/proto/v1/task"
)

const (
	MockValidXAPIKey = "valid-x-api-key"
	validContentType = "application/json"
)

var (
	logger zerolog.Logger

	validBody string
)

func init() {
	logger = logging.NewLogger().With().Str("handler", "test").Logger()

	validBody = fmt.Sprintf(`
	{
		"taskRefId": "%s",
		"taskStatus": "done",
		"UserVar": {
			"orderNumber": "cf0df07b-335a-4344-8221-2fba0d507d26"
		}
	}`,
		primitive.NewObjectID().Hex(),
	)
}

func newTestMileappHandlers(mockClient *tpbmock.MockTaskServiceClient) *MileappHandlers {
	h := NewMileappHandlers(MockValidXAPIKey, mockClient)

	return h
}

func TestHandleStatusUpdate(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := tpbmock.NewMockTaskServiceClient(ctrl)

	t.Run("Success", func(t *testing.T) {
		t.Parallel()

		mockClient.EXPECT().GetOrderTask(gomock.Any(), gomock.Any()).Return(&tpb.GetOrderTaskResponse{}, nil)
		mockClient.EXPECT().UpdateOrderTask(gomock.Any(), gomock.Any()).Return(&tpb.UpdateOrderTaskResponse{}, nil)

		h := newTestMileappHandlers(mockClient)
		w := httptest.NewRecorder()

		updateOrderTaskRequest := []byte(validBody)

		r, err := http.NewRequest(http.MethodPost, "/mileapp/status/picking", bytes.NewBuffer(updateOrderTaskRequest))
		if err != nil {
			t.Fatal(err)
		}

		r.Header.Set("x-api-key", MockValidXAPIKey)
		r.Header.Set("content-type", validContentType)

		router := mux.NewRouter()
		router.HandleFunc("/mileapp/status/{task-type}", h.HandleStatusUpdate)
		router.ServeHTTP(w, r)

		resp := w.Result()
		if gotStatusCode := resp.StatusCode; gotStatusCode != http.StatusOK {
			t.Errorf("HandleStatusUpdate(), got = %v, want = %v", gotStatusCode, http.StatusOK)
		}

		if gotContentType := resp.Header.Get("Content-Type"); gotContentType != validContentType {
			t.Errorf("HandleStatusUpdate(), got = %v, want = %v", gotContentType, validContentType)
		}
	})

	failedCases := []struct {
		name               string
		method             string
		headers            map[string]string
		in                 []byte
		want               *HandleStatusUpdateResponse
		wantHTTPStatusCode int
		taskClientFn       []func(m *tpbmock.MockTaskServiceClient)
	}{
		{
			name:   "MethodNotPost",
			method: http.MethodGet,
			headers: map[string]string{
				"Content-Type": validContentType,
				"X-Api-Key":    MockValidXAPIKey,
			},
			in: []byte(validBody),
			want: &HandleStatusUpdateResponse{
				Message: "expecting http method post, got: GET",
			},
			wantHTTPStatusCode: http.StatusBadRequest,
		},
		{
			name:   "MissingContentType",
			method: http.MethodPost,
			headers: map[string]string{
				"X-Api-Key": MockValidXAPIKey,
			},
			in: []byte(validBody),
			want: &HandleStatusUpdateResponse{
				Message: ErrContenTypeIsRequired.Error(),
			},
			wantHTTPStatusCode: http.StatusBadRequest,
		},
		{
			name:   "InvalidContentType",
			method: http.MethodPost,
			headers: map[string]string{
				"Content-Type": "text/html",
				"X-Api-Key":    MockValidXAPIKey,
			},
			in: []byte(validBody),
			want: &HandleStatusUpdateResponse{
				Message: ErrInvalidContentType.Error(),
			},
			wantHTTPStatusCode: http.StatusBadRequest,
		},
		{
			name:   "BodyNotJSON",
			method: http.MethodPost,
			headers: map[string]string{
				"Content-Type": validContentType,
				"X-Api-Key":    MockValidXAPIKey,
			},
			in: []byte("invalid body"),
			want: &HandleStatusUpdateResponse{
				Message: "invalid request data",
			},
			wantHTTPStatusCode: http.StatusBadRequest,
		},
		{
			name:   "MissingXAPIKey",
			method: http.MethodPost,
			headers: map[string]string{
				"Content-Type": validContentType,
			},
			in: []byte(validBody),
			want: &HandleStatusUpdateResponse{
				Message: ErrXAPIKeyIsRequired.Error(),
			},
			wantHTTPStatusCode: http.StatusBadRequest,
		},
		{
			name:   "InvalidXAPIKey",
			method: http.MethodPost,
			headers: map[string]string{
				"Content-Type": validContentType,
				"X-Api-Key":    "invalid-x-api-key",
			},
			in: []byte(validBody),
			want: &HandleStatusUpdateResponse{
				Message: ErrInvalidXAPIKey.Error(),
			},
			wantHTTPStatusCode: http.StatusBadRequest,
		},
		{
			name:   "EmptyTaskRefID",
			method: http.MethodPost,
			headers: map[string]string{
				"Content-Type": validContentType,
				"X-Api-Key":    MockValidXAPIKey,
			},
			in: []byte(`{
				"taskRefId": "",
				"taskStatus":    "done"
			}`),
			want: &HandleStatusUpdateResponse{
				Message: ErrTaskRefIDIsRequired.Error(),
			},
			wantHTTPStatusCode: http.StatusBadRequest,
		},
		{
			name:   "EmptyStatus",
			method: http.MethodPost,
			headers: map[string]string{
				"Content-Type": validContentType,
				"X-Api-Key":    MockValidXAPIKey,
			},
			in: []byte(`{
				"taskRefId": "1234",
				"taskStatusstatus": "",
				"UserVar": {
					"orderNumber": "cf0df07b-335a-4344-8221-2fba0d507d26"
				}
			}`),
			want: &HandleStatusUpdateResponse{
				Message: ErrStatusIsRequired.Error(),
			},
			wantHTTPStatusCode: http.StatusBadRequest,
		},
	}

	for _, fc := range failedCases {
		fc := fc

		t.Run(fc.name, func(t *testing.T) {
			t.Parallel()

			h := newTestMileappHandlers(mockClient)
			w := httptest.NewRecorder()

			r, err := http.NewRequest(fc.method, "/mileapp/status/picking", bytes.NewBuffer(fc.in))
			if err != nil {
				t.Fatal(err)
			}

			for k, v := range fc.headers {
				r.Header.Set(k, v)
			}

			router := mux.NewRouter()
			router.HandleFunc("/mileapp/status/{task-type}", h.HandleStatusUpdate)
			router.ServeHTTP(w, r)

			gotHTTPStatusCode := w.Result().StatusCode
			if gotHTTPStatusCode != fc.wantHTTPStatusCode {
				t.Errorf("got %d, want %d", gotHTTPStatusCode, fc.wantHTTPStatusCode)
			}

			got := &HandleStatusUpdateResponse{}
			if err := json.NewDecoder(w.Body).Decode(got); err != nil {
				t.Errorf("HandleStatusUpdate() got %v, want nil", err)
			}
			if !cmp.Equal(got, fc.want) {
				t.Errorf("HandleStatusUpdate(), got %v, want %v", got, fc.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		in      *HandleStatusUpdateRequest
		wantErr error
	}{
		{
			name: "Valid",
			in: &HandleStatusUpdateRequest{
				TaskRefID:  "1234",
				TaskStatus: "done",
				UserVar: UserVar{
					OrderNumber: "12345",
				},
			},
			wantErr: nil,
		},
		{
			name: "EmptyTaskRefID",
			in: &HandleStatusUpdateRequest{
				TaskRefID:  "",
				TaskStatus: "ongoing",
				UserVar: UserVar{
					OrderNumber: "12345",
				},
			},
			wantErr: ErrTaskRefIDIsRequired,
		},
		{
			name: "EmptyTaskStatus",
			in: &HandleStatusUpdateRequest{
				TaskRefID:  "1234",
				TaskStatus: "",
				UserVar: UserVar{
					OrderNumber: "12345",
				},
			},
			wantErr: ErrStatusIsRequired,
		},
		{
			name: "InvalidTaskStatus",
			in: &HandleStatusUpdateRequest{
				TaskRefID:  "1234",
				TaskStatus: "whatever",
				UserVar: UserVar{
					OrderNumber: "12345",
				},
			},
			wantErr: ErrInvalidStatus,
		},
		{
			name: "EmptyOrderNumber",
			in: &HandleStatusUpdateRequest{
				TaskRefID:  "1234",
				TaskStatus: "whatever",
				UserVar: UserVar{
					OrderNumber: "",
				},
			},
			wantErr: ErrOrderNumberIsRequired,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.in.Validate(logger); !errors.Is(err, tc.wantErr) {
				t.Errorf("Validate() got %v, want %v", err, tc.wantErr)
			}
		})
	}
}
