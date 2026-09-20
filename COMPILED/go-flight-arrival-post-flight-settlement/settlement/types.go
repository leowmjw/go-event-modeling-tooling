package settlement

const (
	TaskQueue                = "flight-settlement-v1"
	ParentWorkflowName       = "FlightSettlementWorkflowV1"
	OperationsWorkflowName   = "OperationsWorkflowV1"
	CompensationWorkflowName = "CompensationWorkflowV1"
	FinanceWorkflowName      = "FinanceWorkflowV1"
	MarketingWorkflowName    = "MarketingWorkflowV1"
	RecoveryResponseSignal   = "respond_to_recovery_offer"
	CampaignDashboardQuery   = "marketing_campaign_dashboard"
	ConversionReportQuery    = "marketing_conversion_report"
)

type Flight struct {
	FlightID         string `json:"flightId"`
	Origin           string `json:"origin"`
	Destination      string `json:"destination"`
	ScheduledArrival string `json:"scheduledArrival"`
	CurrentStatus    string `json:"currentStatus"`
	AircraftID       string `json:"aircraftId"`
}
type RecordWheelsDownCommand struct {
	FlightID        string `json:"flightId"`
	WheelsTouchTime string `json:"wheelsTouchTime"`
	AircraftID      string `json:"aircraftId"`
}
type WheelsDownRecorded struct {
	FlightID        string `json:"flightId"`
	WheelsTouchTime string `json:"wheelsTouchTime"`
	AircraftID      string `json:"aircraftId"`
	BoundedContext  string `json:"boundedContext"`
}
type RecordGateArrivalCommand struct {
	FlightID         string `json:"flightId"`
	GateDoorOpenTime string `json:"gateDoorOpenTime"`
	GateID           string `json:"gateId"`
}
type GateOpened struct {
	FlightID               string `json:"flightId"`
	ScheduledArrival       string `json:"scheduledArrival"`
	GateDoorOpenTime       string `json:"gateDoorOpenTime"`
	GateID                 string `json:"gateId"`
	OnTimeThresholdMinutes int    `json:"onTimeThresholdMinutes"`
	BoundedContext         string `json:"boundedContext"`
}
type ArrivalBoard struct {
	FlightID         string `json:"flightId"`
	GateID           string `json:"gateId"`
	GateDoorOpenTime string `json:"gateDoorOpenTime"`
	DisplayStatus    string `json:"displayStatus"`
}
type FlightClosedOnTime struct {
	FlightID string `json:"flightId"`
	ClosedAt string `json:"closedAt"`
}
type FlightClosureRecord struct {
	FlightID string `json:"flightId"`
	Status   string `json:"status"`
	ClosedAt string `json:"closedAt"`
}
type OperationsInput struct {
	Flight      Flight                   `json:"flight"`
	WheelsDown  RecordWheelsDownCommand  `json:"recordWheelsDown"`
	GateArrival RecordGateArrivalCommand `json:"recordGateArrival"`
	OnTime      bool                     `json:"onTime"`
}
type OperationsResult struct {
	Event         string               `json:"event"`
	WheelsDown    WheelsDownRecorded   `json:"wheelsDown"`
	GateOpened    *GateOpened          `json:"gateOpened,omitempty"`
	ArrivalBoard  *ArrivalBoard        `json:"arrivalBoard,omitempty"`
	ClosedOnTime  *FlightClosedOnTime  `json:"closedOnTime,omitempty"`
	ClosureRecord *FlightClosureRecord `json:"closureRecord,omitempty"`
}

type EvaluateDelayCommand struct {
	FlightID               string `json:"flightId"`
	ScheduledArrival       string `json:"scheduledArrival"`
	ActualArrivalTime      string `json:"actualArrivalTime"`
	OnTimeDefinition       string `json:"onTimeDefinition"`
	OnTimeThresholdMinutes int    `json:"onTimeThresholdMinutes"`
}
type DelayEvaluated struct {
	FlightID               string `json:"flightId"`
	DelayMinutes           int    `json:"delayMinutes"`
	IsDelayed              bool   `json:"isDelayed"`
	OnTimeThresholdMinutes int    `json:"onTimeThresholdMinutes"`
	BoundedContext         string `json:"boundedContext"`
}
type PassengerEligibilityVerified struct {
	FlightID      string   `json:"flightId"`
	EligibleIDs   []string `json:"eligibleIds"`
	IneligibleIDs []string `json:"ineligibleIds"`
}
type CompensationCalculated struct {
	FlightID                string             `json:"flightId"`
	TotalCompensationAmount float64            `json:"totalCompensationAmount"`
	PerPassengerAmount      float64            `json:"perPassengerAmount"`
	Currency                string             `json:"currency"`
	EligiblePassengerIDs    []string           `json:"eligiblePassengerIds"`
	BillingAccounts         []PassengerAccount `json:"billingAccounts"`
}
type PassengerAccount struct {
	PassengerID string `json:"passengerId"`
	AccountID   string `json:"accountId"`
}
type DelayClaimPassenger struct {
	PassengerID        string  `json:"passengerId"`
	CompensationAmount float64 `json:"compensationAmount"`
}
type DelayClaimSummary struct {
	FlightID             string                `json:"flightId"`
	DelayMinutes         int                   `json:"delayMinutes"`
	EligiblePassengers   []DelayClaimPassenger `json:"eligiblePassengers"`
	IneligiblePassengers []string              `json:"ineligiblePassengers"`
	TotalCompensation    float64               `json:"totalCompensation"`
	Currency             string                `json:"currency"`
	Status               string                `json:"status"`
}
type CompensationCaseClosed struct {
	FlightID string `json:"flightId"`
	Reason   string `json:"reason"`
	ClosedAt string `json:"closedAt"`
}
type PartialDisbursementRejected struct {
	FlightID             string   `json:"flightId"`
	RejectedPassengerIDs []string `json:"rejectedPassengerIds"`
	RejectedAmount       float64  `json:"rejectedAmount"`
	Reason               string   `json:"reason"`
	EscalatedAt          string   `json:"escalatedAt"`
}
type ClaimEscalated struct {
	FlightID     string   `json:"flightId"`
	PassengerIDs []string `json:"passengerIds"`
	Amount       float64  `json:"amount"`
	EscalatedAt  string   `json:"escalatedAt"`
}
type CompensationInput struct {
	GateOpened      *GateOpened                  `json:"gateOpened,omitempty"`
	ClosedOnTime    *FlightClosedOnTime          `json:"closedOnTime,omitempty"`
	PartialRejected *PartialDisbursementRejected `json:"partialRejected,omitempty"`
	PassengerIDs    []string                     `json:"passengerIds,omitempty"`
}
type CompensationResult struct {
	Event       string                        `json:"event"`
	Delay       *DelayEvaluated               `json:"delay,omitempty"`
	Eligibility *PassengerEligibilityVerified `json:"eligibility,omitempty"`
	Calculated  *CompensationCalculated       `json:"calculated,omitempty"`
	Summary     *DelayClaimSummary            `json:"summary,omitempty"`
	Closed      *CompensationCaseClosed       `json:"closed,omitempty"`
	Escalated   *ClaimEscalated               `json:"escalated,omitempty"`
}

type ApproveDisbursementCommand struct {
	FlightID             string             `json:"flightId"`
	DisbursementID       string             `json:"disbursementId"`
	TotalAmount          float64            `json:"totalAmount"`
	PerPassengerAmount   float64            `json:"perPassengerAmount"`
	EligiblePassengerIDs []string           `json:"eligiblePassengerIds"`
	BillingAccounts      []PassengerAccount `json:"billingAccounts"`
	ApprovedAt           string             `json:"approvedAt"`
}
type DisbursementApproved struct {
	FlightID       string  `json:"flightId"`
	DisbursementID string  `json:"disbursementId"`
	TotalAmount    float64 `json:"totalAmount"`
	ApprovedAt     string  `json:"approvedAt"`
	BoundedContext string  `json:"boundedContext"`
}
type PayoutIssued struct {
	DisbursementID    string   `json:"disbursementId"`
	FlightID          string   `json:"flightId"`
	PaymentRails      string   `json:"paymentRails"`
	ScheduledDate     string   `json:"scheduledDate"`
	TotalAmount       float64  `json:"totalAmount"`
	BillingAccountIDs []string `json:"billingAccountIds"`
	TravelerIDs       []string `json:"travelerIds"`
	SegmentTag        string   `json:"segmentTag"`
}
type FinanceSettlementReport struct {
	DisbursementID  string          `json:"disbursementId"`
	FlightID        string          `json:"flightId"`
	TotalAmount     float64         `json:"totalAmount"`
	PaymentRails    string          `json:"paymentRails"`
	ScheduledDate   string          `json:"scheduledDate"`
	Status          string          `json:"status"`
	BillingAccounts []AccountAmount `json:"billingAccounts"`
}
type AccountAmount struct {
	AccountID string  `json:"accountId"`
	Amount    float64 `json:"amount"`
}
type FinanceInput struct {
	Compensation  CompensationCalculated `json:"compensation"`
	ApprovedAt    string                 `json:"approvedAt"`
	ScheduledDate string                 `json:"scheduledDate"`
}
type PayoutFailed struct {
	DisbursementID        string  `json:"disbursementId"`
	FailureCode           string  `json:"failureCode"`
	TotalAmount           float64 `json:"totalAmount"`
	FailedAt              string  `json:"failedAt"`
	RecoveryPaymentRails  string  `json:"recoveryPaymentRails"`
	RecoveryScheduledDate string  `json:"recoveryScheduledDate"`
}
type ReverseAndReissueCommand struct {
	DisbursementID  string  `json:"disbursementId"`
	FailureCode     string  `json:"failureCode"`
	TotalAmount     float64 `json:"totalAmount"`
	ReversedAt      string  `json:"reversedAt"`
	NewPaymentRails string  `json:"newPaymentRails"`
	ScheduledDate   string  `json:"scheduledDate"`
}
type PayoutReversed struct {
	DisbursementID string `json:"disbursementId"`
	FailureCode    string `json:"failureCode"`
	ReversedAt     string `json:"reversedAt"`
}
type PayoutReissued struct {
	DisbursementID  string  `json:"disbursementId"`
	NewPaymentRails string  `json:"newPaymentRails"`
	ScheduledDate   string  `json:"scheduledDate"`
	TotalAmount     float64 `json:"totalAmount"`
}
type PayoutRecoveryResult struct {
	Event    string          `json:"event"`
	Reversed *PayoutReversed `json:"payoutReversed,omitempty"`
	Reissued *PayoutReissued `json:"payoutReissued,omitempty"`
	Reason   string          `json:"reason,omitempty"`
}
type PayoutRecoveryReport struct {
	DisbursementID string `json:"disbursementId"`
	Status         string `json:"status"`
	ScheduledDate  string `json:"scheduledDate"`
}
type FinanceResult struct {
	Event           string                       `json:"event"`
	Approved        *DisbursementApproved        `json:"approved,omitempty"`
	PartialRejected *PartialDisbursementRejected `json:"partialRejected,omitempty"`
	Payout          *PayoutIssued                `json:"payout,omitempty"`
	Report          *FinanceSettlementReport     `json:"report,omitempty"`
	Recovery        *PayoutRecoveryResult        `json:"recovery,omitempty"`
}

type TagDelayedTravelersCommand struct {
	FlightID           string   `json:"flightId"`
	TravelerIDs        []string `json:"travelerIds"`
	SegmentTag         string   `json:"segmentTag"`
	CustomerDefinition string   `json:"customerDefinition"`
}
type DelayedTravelerTagged struct {
	FlightID      string `json:"flightId"`
	SegmentTag    string `json:"segmentTag"`
	TravelerCount int    `json:"travelerCount"`
}
type RecoveryOfferSent struct {
	SegmentTag  string   `json:"segmentTag"`
	OfferID     string   `json:"offerId"`
	TravelerIDs []string `json:"travelerIds"`
	Channel     string   `json:"channel"`
	OffersSent  int      `json:"offersSent"`
	SentAt      string   `json:"sentAt"`
}
type RespondToRecoveryOfferCommand struct {
	OfferID     string `json:"offerId"`
	TravelerID  string `json:"travelerId"`
	Response    string `json:"response"`
	RespondedAt string `json:"respondedAt"`
}
type RecoveryOfferResponse struct {
	Event       string `json:"event"`
	OfferID     string `json:"offerId"`
	TravelerID  string `json:"travelerId"`
	RespondedAt string `json:"respondedAt"`
}
type MarketingCampaignDashboard struct {
	SegmentTag        string  `json:"segmentTag"`
	ActiveCampaigns   int     `json:"activeCampaigns"`
	TargetedTravelers int     `json:"targetedTravelers"`
	OffersSent        int     `json:"offersSent"`
	OffersAccepted    int     `json:"offersAccepted"`
	OffersDeclined    int     `json:"offersDeclined"`
	ConversionRate    float64 `json:"conversionRate"`
}
type MarketingConversionReport struct {
	OfferID       string `json:"offerId"`
	AcceptedCount int    `json:"acceptedCount"`
	DeclinedCount int    `json:"declinedCount"`
}
type MarketingInput struct {
	Payout  PayoutIssued `json:"payout"`
	OfferID string       `json:"offerId"`
	Channel string       `json:"channel"`
	SentAt  string       `json:"sentAt"`
}
type MarketingResult struct {
	Event      string                     `json:"event"`
	Tagged     DelayedTravelerTagged      `json:"tagged"`
	Sent       RecoveryOfferSent          `json:"sent"`
	Response   RecoveryOfferResponse      `json:"response"`
	Dashboard  MarketingCampaignDashboard `json:"dashboard"`
	Conversion MarketingConversionReport  `json:"conversion"`
}

type ParentInput struct {
	Operations    OperationsInput `json:"operations"`
	PassengerIDs  []string        `json:"passengerIds"`
	ApprovedAt    string          `json:"approvedAt"`
	ScheduledDate string          `json:"scheduledDate"`
	OfferID       string          `json:"offerId"`
	Channel       string          `json:"channel"`
	SentAt        string          `json:"sentAt"`
}
type ParentResult struct {
	Operations   OperationsResult   `json:"operations"`
	Compensation CompensationResult `json:"compensation"`
	Finance      *FinanceResult     `json:"finance,omitempty"`
	Marketing    *MarketingResult   `json:"marketing,omitempty"`
}
