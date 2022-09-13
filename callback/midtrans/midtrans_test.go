package midtrans

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"

	opbmock "github.com/dropezy/proto/mock/order"
	tpbmock "github.com/dropezy/proto/mock/task"
)

func TestHandleTransactionUpdate(t *testing.T) {
	serverKey := "askvnoibnosifnboseofinbofinfgbiufglnbfg"
	t.Parallel()

	ctrl := gomock.NewController(t)
	orderClient := opbmock.NewMockOrderServiceClient(ctrl)
	taskClient := tpbmock.NewMockTaskServiceClient(ctrl)

	t.Run("Failed_UnsupportedPaymentType", func(t *testing.T) {
		bReq := func() []byte {
			req := UpdateTransactionRequest{
				OrderID:       "1111",
				TransactionID: uuid.NewString(),
				GrossAmount:   "100000.00",
				SignatureKey:  "edc076b21793ebe3e17926350f5b8ae67d902fe657b3d0aa31b932d5c127e2375d308a2bc94f3265ac2d80a1f181a79b997ac178a236fcff35af263fc4d4c231",
				StatusCode:    "200",
			}
			breq, err := json.Marshal(req)
			if err != nil {
				t.Fatal(err)
			}
			return breq
		}

		w := httptest.NewRecorder()
		r, err := http.NewRequest(http.MethodPost, TransactionUpdatePath, bytes.NewBuffer(bReq()))
		if err != nil {
			t.Fatal(err)
		}

		r.Header.Set("Content-Type", "application/json")
		h, err := NewHandler(serverKey, "localhost", "localhost", orderClient, taskClient)
		if err != nil {
			t.Fatal(err)
		}

		handler := http.HandlerFunc(h.HandleTransactionUpdate)
		handler.ServeHTTP(w, r)

		resp := w.Result()
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("want http 500, got : %v", resp.StatusCode)
		}

	})
}
