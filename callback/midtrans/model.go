package midtrans

// UpdateTransactionRequest holds all data used for charge response and payment notification request
// from midtrans.
type UpdateTransactionRequest struct {
	// TransactionTime is Timestamp of transaction in ISO 8601 format. Time Zone: GMT+7.
	TransactionTime string `json:"transaction_time"`
	// TransactionStatus Status of GoPay transaction. Possible values are
	// pending, settlement, expire, deny.
	TransactionStatus string `json:"transaction_status"`
	// TransactionID is Transaction ID given by payment provider.
	TransactionID string `json:"transaction_id"`
	// StatusMessage is Description of transaction charge result.
	StatusMessage string `json:"status_message"`
	// StatusCode is Status code of transaction charge result.
	StatusCode string `json:"status_code"`
	// SignatureKey is additional security used to verify the notification request from midtrans.
	// This is combination from order_id. status_code, gross_amount and server_key.
	SignatureKey string `json:"signature_key"`
	// SettlementTime is the time at which the transaction status was confirmed became "settlement".
	SettlementTime string `json:"settlement_time"`
	// PaymentType is Transaction payment method.
	PaymentType string `json:"payment_type"`
	// OrderID is Order task ID specified by dropezy.
	OrderID string `json:"order_id"`
	// MerchantID is merchant ID for dropezy.
	MerchantID string `json:"merchant_id"`
	// GrossAmount is Total amount of transaction in IDR.
	GrossAmount string `json:"gross_amount"`
	// FraudStatus is detection result by Midtrans Fraud Detection System (FDS). Possible values are
	// accept : Approved by FDS.
	// challenge: Questioned by FDS. Note: Approve transaction to accept it or transaction gets automatically canceled during settlement.
	// deny: Denied by FDS. Transaction automatically failed.
	FraudStatus string `json:"fraud_status"`
	// Currency is currency used in the transaction.
	Currency string `json:"currency"`
}
