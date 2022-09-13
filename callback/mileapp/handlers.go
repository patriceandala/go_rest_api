package mileapp

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/dropezy/internal/logging"
	tpb "github.com/dropezy/proto/v1/task"
)

const handlerName = "mileapp"

type MileappHandlers struct {
	grpcClient tpb.TaskServiceClient
	authKey    string
}

func NewMileappHandlers(authKey string, client tpb.TaskServiceClient) *MileappHandlers {
	return &MileappHandlers{
		grpcClient: client,
		authKey:    authKey,
	}
}

// HandlerStatusUpdate handle callback from MileApp to update the delivery status, method is POST
func (m *MileappHandlers) HandleStatusUpdate(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context()).With().Str("handler", handlerName).Logger()

	logger.Info().Msgf("received status update from: %s", r.RemoteAddr)

	var taskType tpb.OrderTaskType
	switch task := mux.Vars(r)["task-type"]; task {
	case taskTypePicking:
		taskType = tpb.OrderTaskType_ORDER_TASK_TYPE_PICKING
	case taskTypePacking:
		taskType = tpb.OrderTaskType_ORDER_TASK_TYPE_PACKING
	case taskTypeShipping:
		taskType = tpb.OrderTaskType_ORDER_TASK_TYPE_SHIPPING
	case taskTypeDelivery:
		taskType = tpb.OrderTaskType_ORDER_TASK_TYPE_DELIVERY
	default:
		err := fmt.Errorf("unsupported task type: %s", task)
		logger.Err(err).Send()
		m.responseJSON(logger, w, http.StatusBadRequest, err.Error())
		return
	}

	logger = logger.With().Str("taskType", taskType.String()).Logger()

	if r.Method != http.MethodPost {
		err := fmt.Errorf("expecting http method post, got: %s", r.Method)
		logger.Err(err).Send()
		m.responseJSON(logger, w, http.StatusBadRequest, err.Error())
		return
	}
	if err := m.validateHeaders(logger, r.Header); err != nil {
		m.responseJSON(logger, w, http.StatusBadRequest, err.Error())
		return
	}

	req := &HandleStatusUpdateRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		logger.Err(err).Msg("failed to decode request data")
		m.responseJSON(logger, w, http.StatusBadRequest, "invalid request data")
		return
	}

	// check if the request contains all required fields
	if err := req.Validate(logger); err != nil {
		logger.Err(err).Send()
		m.responseJSON(logger, w, http.StatusBadRequest, err.Error())
		return
	}

	logger = logger.With().Fields(map[string]interface{}{
		"taskRefId":   req.TaskRefID,
		"taskStatus":  req.TaskStatus,
		"orderNumber": req.UserVar.OrderNumber,
	}).Logger()

	tasks, err := m.grpcClient.GetOrderTask(r.Context(), &tpb.GetOrderTaskRequest{
		OrderId: req.UserVar.OrderNumber,
	})
	if err != nil {
		logger.Err(err).Msg("failed to get order task")
		m.responseJSON(logger, w, http.StatusInternalServerError, "failed to update order task")
		return
	}

	orderTask := &tpb.OrderTask{}
	for _, t := range tasks.Tasks {
		if t.TaskType == taskType {
			orderTask = t
			break
		}
	}

	logger = logger.With().Fields(map[string]interface{}{
		"taskID": orderTask.TaskId,
	}).Logger()

	// mileapp sometimes send the callback twice.
	// ignore if we already updated the task state to done.
	if orderTask.State == tpb.OrderTaskState_ORDER_TASK_STATE_SUCCESS {
		logger.Info().Msg("order task is already marked successfull, ignoring")
		m.responseJSON(logger, w, http.StatusOK, "success")
		return
	}

	updateReq := req.ToPB()
	updateReq.TaskId = orderTask.TaskId

	// send driver info so we can add it to order data
	// when taskType is shipping and its status is ongoing
	if orderTask.GetTaskType() == tpb.OrderTaskType_ORDER_TASK_TYPE_SHIPPING &&
		req.TaskStatus == statusOngoing {

		updateReq.AdditionalData = map[string]string{
			"driver_name":  req.AssignedTo.FullName,
			"driver_phone": req.UserVar.DriverPhone,
		}
	}

	// send recipient info so we can add it to order data
	// when taskType is delivery and its done
	if orderTask.GetTaskType() == tpb.OrderTaskType_ORDER_TASK_TYPE_DELIVERY &&
		req.TaskStatus == statusDone {

		updateReq.AdditionalData = map[string]string{
			"receiver_role": req.UserVar.Receiver,
			"receiver_name": req.UserVar.ReceiverName,
		}
	}

	// using grpc to store the status update to the database, the grpc response is currently empty
	if _, err := m.grpcClient.UpdateOrderTask(r.Context(), updateReq); err != nil {
		logger.Err(err).Msg("failed to update order task")
		m.responseJSON(logger, w, http.StatusInternalServerError, "failed to update order task")
		return
	}

	logger.Info().Msg("successfully processing update task status")
	m.responseJSON(logger, w, http.StatusOK, "success")
}

// responseJSON is used for responsding to the http caller
func (m *MileappHandlers) responseJSON(logger zerolog.Logger, w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")

	res, err := json.Marshal(&HandleStatusUpdateResponse{Message: message})
	if err != nil {
		logger.Err(ErrMarshallingUnsuccessful).Msg(ErrMarshallingUnsuccessful.Error())
	}

	w.WriteHeader(statusCode)

	if _, err = w.Write(res); err != nil {
		logger.Err(ErrWriteToResponseUnsuccessful).Msg(ErrWriteToResponseUnsuccessful.Error())
	}
}

// validateHeaders to check if Content-Type and X-Api-Key is given and not empty.
func (m *MileappHandlers) validateHeaders(logger zerolog.Logger, h http.Header) error {

	ct := h.Get("content-type")
	if ct == "" {
		logger.Err(ErrContenTypeIsRequired).Msg(ErrContenTypeIsRequired.Error())
		return ErrContenTypeIsRequired
	}

	if ct != "application/json" {
		logger.Err(ErrInvalidContentType).Msg(ErrInvalidContentType.Error())
		return ErrInvalidContentType
	}

	apiKey := h.Get("x-api-key")
	if apiKey == "" {
		logger.Err(ErrXAPIKeyIsRequired).Msg(ErrXAPIKeyIsRequired.Error())
		return ErrXAPIKeyIsRequired
	}

	if apiKey != m.authKey {
		logger.Err(ErrInvalidXAPIKey).Msg(ErrInvalidXAPIKey.Error())
		return ErrInvalidXAPIKey
	}

	return nil
}

type UserVar struct {
	OrderNumber  string `json:"orderNumber"`
	Receiver     string `json:"receiver"`
	ReceiverName string `json:"receiverName"`
	DriverPhone  string `json:"driverPhone"`
}

type HandleStatusUpdateRequest struct {
	TaskRefID  string     `json:"taskRefId"`
	TaskStatus string     `json:"taskStatus"`
	UserVar    UserVar    `json:"UserVar"`
	AssignedTo AssignedTo `json:"assignedTo"`
}

type AssignedTo struct {
	FullName string `json:"full_name"`
}

type HandleStatusUpdateResponse struct {
	Message string `json:"message"`
}

// Validate check all HandleStatusUpdateRequest fields, returns error if empty
func (h *HandleStatusUpdateRequest) Validate(logger zerolog.Logger) error {
	if h.TaskRefID == "" {
		return ErrTaskRefIDIsRequired
	}
	if h.UserVar.OrderNumber == "" {
		return ErrOrderNumberIsRequired
	}

	switch h.TaskStatus {
	case "":
		return ErrStatusIsRequired
	case statusOngoing, statusDone:
		// continue
	default:
		logger.Error().Msgf("got task status: %s", h.TaskStatus)
		return ErrInvalidStatus
	}
	return nil
}

func (h *HandleStatusUpdateRequest) ToPB() *tpb.UpdateOrderTaskRequest {
	req := &tpb.UpdateOrderTaskRequest{}
	switch h.TaskStatus {
	case statusOngoing, statusDone:
		// mileapp shared 1 task into 2 states for pickup and delivery flow.
		// in case for pickup the task status will be ongoing, but we should
		// map it to success anyway as internally for us its 2 separate tasks.
		req.State = tpb.OrderTaskState_ORDER_TASK_STATE_SUCCESS
	}
	return req
}
