package midtrans

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/dropezy/internal/logging"
	"github.com/dropezy/storefront-backend/internal/integrations/payment"
	"github.com/dropezy/storefront-backend/internal/integrations/payment/midtrans/auth"
	"github.com/dropezy/storefront-backend/internal/integrations/payment/midtrans/transaction"

	// protobuf
	opb "github.com/dropezy/proto/v1/order"
	tpb "github.com/dropezy/proto/v1/task"
)

const handlerName = "midtrans"

const (
	TransactionUpdatePath = "/midtrans/transaction-update"

	defaultContextTimeout = 15 * time.Second

	PendingTransactionStatus           = "pending"
	AuthorizedTransactionStatus        = "authorized"
	CaptureTransactionStatus           = "capture"
	SettlementTransactionStatus        = "settlement"
	DenyTransactionStatus              = "deny"
	CancelTransactionStatus            = "cancel"
	RefundTransactionStatus            = "refund"
	PartialRefundTransactionStatus     = "partial_refund"
	ChargebackTransactionStatus        = "chargeback"
	PartialChargebackTransactionStatus = "partial_chargeback"
	ExpireTransactionStatus            = "expire"
	FailureTransactionStatus           = "failure"

	FraudStatusAccept = "accept"
)

type Handler struct {
	serverKey    string
	chargeURL    string
	getStatusURL string

	orderService opb.OrderServiceClient
	taskService  tpb.TaskServiceClient
}

func NewHandler(serverKey string,
	chargeURL, getStatusURL string,
	orderService opb.OrderServiceClient,
	taskService tpb.TaskServiceClient) (*Handler, error) {
	if serverKey == "" {
		return nil, errors.New("serverKey not found")
	}

	return &Handler{
		serverKey:    serverKey,
		chargeURL:    chargeURL,
		getStatusURL: getStatusURL,

		orderService: orderService,
		taskService:  taskService,
	}, nil
}

// HandlePaymentNotification handle payment notification from midtrans to
// update our payment status. The flow on this are:
//     1. Our system receive the request
//     2. Validate the payload and signature
//     3. Send GetTransactionStatus Request to midtrans to get the reliable
//        status for our transaction
//        (ref# https://api-docs.midtrans.com/?go#receiving-notifications)
//
// TODO (novian): Add call to geofencing API for success payment
func (h *Handler) HandleTransactionUpdate(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context()).With().Str("handler", handlerName).Logger()

	ctx, cancelFn := context.WithTimeout(r.Context(), defaultContextTimeout)
	defer cancelFn()

	if r.Method != http.MethodPost {
		err := fmt.Errorf("expecting http method post, got: %s", r.Method)
		logger.Err(err).Send()
	}

	if err := validateHeaders(logger, r.Header); err != nil {
		writeJSONResponse(w, http.StatusBadRequest)
		return
	}

	req := &UpdateTransactionRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		logger.Err(err).Msg("failed to decode request data")
		writeJSONResponse(w, http.StatusBadRequest)
		return
	}

	logger = logger.With().Fields(map[string]interface{}{
		"task_id":         req.OrderID,
		"transaction_id":  req.TransactionID,
		"signature":       req.SignatureKey,
		"request_payload": req,
	}).Logger()

	if err := auth.ValidateCallbackSignature(
		req.SignatureKey, req.OrderID, req.StatusCode, req.GrossAmount, h.serverKey); err != nil {
		logger.Err(ErrInvalidSignature).Msg("invalid callbak signature")
		writeJSONResponse(w, http.StatusBadRequest)
		return
	}

	// only check for pending transaction because it will be skipped.
	// the other status will be check below.
	if strings.ToLower(req.TransactionStatus) == PendingTransactionStatus {
		writeJSONResponse(w, http.StatusOK)
		return
	}

	transactionGetter, err := h.initializeTransactionGetter(logger, req)
	if err != nil {
		writeJSONResponse(w, http.StatusInternalServerError)
		return
	}

	// ONLY USE REQUEST UNTIL THIS POINT.
	// FOR THE REST, WE WILL USE THE DATA FROM getTransactionStatus RESPONSE!!!
	trx, err := transactionGetter.GetTransactionStatus(req.OrderID)
	if err != nil {
		logger.Err(err).Msg("failed to get transaction from midtrans API")
		// return http 400 to trigger retry from midtrans system.
		// refer to: https://api-docs.midtrans.com/?go#best-practices-to-handle-notification
		writeJSONResponse(w, http.StatusBadRequest)
		return
	}

	// get order id from order task
	tasks, err := h.taskService.GetOrderTask(ctx, &tpb.GetOrderTaskRequest{
		TaskId: req.OrderID,
	})
	if err != nil {
		logger.Err(err).Msg("invalid task")
		writeJSONResponse(w, http.StatusInternalServerError)
		return
	}

	orderTask := &tpb.OrderTask{}
	for _, t := range tasks.Tasks {
		if t.TaskType == tpb.OrderTaskType_ORDER_TASK_TYPE_PAYMENT {
			orderTask = t
			break
		}
	}

	// prevent update to already success tasks.
	if orderTask.State == tpb.OrderTaskState_ORDER_TASK_STATE_SUCCESS {
		logger.Info().Msg("order task is already marked successfull, ignoring")
		writeJSONResponse(w, http.StatusOK)
		return
	}

	logger = logger.With().Fields(map[string]interface{}{
		"order_id": orderTask.OrderId,
	}).Logger()

	getRes, err := h.orderService.Get(ctx, &opb.GetRequest{OrderId: orderTask.OrderId})
	if err != nil {
		logger.Err(err).Msg("invalid order")
		writeJSONResponse(w, http.StatusInternalServerError)
		return
	}
	order := getRes.GetOrderData().Order

	// check the transaction status should not success or failed.
	// we don't want to update the transaction that already failed or success.
	switch order.State {
	case opb.OrderState_ORDER_STATE_PAID,
		opb.OrderState_ORDER_STATE_CANCELLED,
		opb.OrderState_ORDER_STATE_DONE:
		logger = logger.With().Fields(map[string]interface{}{
			"order_state": order.GetState().String(),
		}).Logger()
		logger.Err(payment.ErrInvalidOrder).Msg("invalid order state")
		writeJSONResponse(w, http.StatusBadRequest)
		return
	}

	logger = logger.With().Fields(map[string]interface{}{
		"transaction_code":         trx.StatusCode,
		"transaction_status":       trx.TransactionStatus,
		"transaction_fraud_status": trx.FraudStatus,
	}).Logger()

	updateFn := func(s tpb.OrderTaskState) error {
		logger.Info().Msg("updating order task")
		if _, err := h.taskService.UpdateOrderTask(ctx, &tpb.UpdateOrderTaskRequest{
			TaskId: orderTask.TaskId,
			State:  s,
		}); err != nil {
			return err
		}
		logger.Info().Msg("successfully updating order task")
		return nil
	}

	switch strings.ToLower(trx.TransactionStatus) {
	case CaptureTransactionStatus, SettlementTransactionStatus:
		if trx.FraudStatus != "" && strings.ToLower(trx.FraudStatus) != FraudStatusAccept {
			if err := updateFn(tpb.OrderTaskState_ORDER_TASK_STATE_FAILED); err != nil {
				logger.Err(err).Msg("failed to update failed task")
				writeJSONResponse(w, http.StatusInternalServerError)
				return
			}
		}

		// capture for VA and settlement for Gopay
		if err := updateFn(tpb.OrderTaskState_ORDER_TASK_STATE_SUCCESS); err != nil {
			logger.Err(err).Msg("failed to update success task")
			writeJSONResponse(w, http.StatusInternalServerError)
			return
		}
	case ExpireTransactionStatus, FailureTransactionStatus,
		CancelTransactionStatus, DenyTransactionStatus:
		if err := updateFn(tpb.OrderTaskState_ORDER_TASK_STATE_FAILED); err != nil {
			logger.Err(err).Msg("failed to update failed task")
			writeJSONResponse(w, http.StatusInternalServerError)
			return
		}
	}

	logger.Info().Msg("successfully processing update transaction status request")
	writeJSONResponse(w, http.StatusOK)
}

func (h *Handler) initializeTransactionGetter(logger zerolog.Logger, req *UpdateTransactionRequest) (payment.TransactionGetter, error) {
	var transactionGetter payment.TransactionGetter
	var err error
	switch req.PaymentType {
	case payment.PaymentMethod_Gopay, payment.PaymentMethod_VirtualAccount:
		transactionGetter, err = transaction.NewTransaction(logger, h.chargeURL, h.getStatusURL, h.serverKey)
		if err != nil {
			logger.Err(ErrInternalServerError).Msg("error initialize transactionGetter")
		}
	default:
		err = ErrUnsupportedPaymentMethod
		logger.Err(err).Msg("invalid payment type")
	}
	return transactionGetter, err
}
