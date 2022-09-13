package mileapp

// supported mileapp task type on our end
const (
	taskTypePicking  = "picking"
	taskTypePacking  = "packing"
	taskTypeShipping = "shipping"
	taskTypeDelivery = "delivery"
)

// expected task update status from mileapp
const (
	statusOngoing = "ongoing"
	statusDone    = "done"
)
