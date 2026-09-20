package settlement

import (
	"fmt"
	"strings"
	"time"
)

func ProjectFlightStatus(f Flight) Flight { return f }
func DecideRecordWheelsDown(c RecordWheelsDownCommand, alreadyRecorded bool) (string, *WheelsDownRecorded) {
	if alreadyRecorded {
		return "WheelsDownRejected", nil
	}
	e := WheelsDownRecorded{c.FlightID, c.WheelsTouchTime, c.AircraftID, "Operations"}
	return "WheelsDownRecorded", &e
}
func DecideRecordGateArrival(c RecordGateArrivalCommand, wheelsDown bool, scheduled string) (string, *GateOpened) {
	if !wheelsDown {
		return "GateArrivalRejected", nil
	}
	e := GateOpened{c.FlightID, scheduled, c.GateDoorOpenTime, c.GateID, 15, "Operations"}
	return "GateOpened", &e
}
func ProjectArrivalBoard(e GateOpened) ArrivalBoard {
	return ArrivalBoard{e.FlightID, e.GateID, e.GateDoorOpenTime, "ARRIVED"}
}
func TranslateOntimeVerifier(e WheelsDownRecorded) FlightClosedOnTime {
	return FlightClosedOnTime{FlightID: e.FlightID, ClosedAt: e.WheelsTouchTime}
}
func DecideCloseFlightOnTime(e WheelsDownRecorded) FlightClosedOnTime {
	return TranslateOntimeVerifier(e)
}
func ProjectFlightClosureRecord(e FlightClosedOnTime) FlightClosureRecord {
	return FlightClosureRecord{e.FlightID, "on_time", e.ClosedAt}
}

func TranslateCompensationProcessor(e GateOpened) EvaluateDelayCommand {
	return EvaluateDelayCommand{e.FlightID, e.ScheduledArrival, e.GateDoorOpenTime, "gate_door_open", e.OnTimeThresholdMinutes}
}
func DecideEvaluateDelay(c EvaluateDelayCommand) (DelayEvaluated, error) {
	scheduled, err := time.Parse(time.RFC3339, c.ScheduledArrival)
	if err != nil {
		return DelayEvaluated{}, err
	}
	actual, err := time.Parse(time.RFC3339, c.ActualArrivalTime)
	if err != nil {
		return DelayEvaluated{}, err
	}
	minutes := int(actual.Sub(scheduled).Minutes())
	return DelayEvaluated{c.FlightID, minutes, minutes > c.OnTimeThresholdMinutes, c.OnTimeThresholdMinutes, "Compensation"}, nil
}
func DecideVerifyPassengerEligibility(flightID string, ids, previouslyPaid []string) PassengerEligibilityVerified {
	paid := make(map[string]bool, len(previouslyPaid))
	for _, id := range previouslyPaid {
		paid[id] = true
	}
	r := PassengerEligibilityVerified{FlightID: flightID, EligibleIDs: []string{}, IneligibleIDs: []string{}}
	for _, id := range ids {
		if paid[id] {
			r.IneligibleIDs = append(r.IneligibleIDs, id)
		} else {
			r.EligibleIDs = append(r.EligibleIDs, id)
		}
	}
	return r
}
func DecideCalculateCompensation(e PassengerEligibilityVerified) CompensationCalculated {
	const amount = 170.0
	accounts := make([]PassengerAccount, 0, len(e.EligibleIDs))
	for _, id := range e.EligibleIDs {
		accounts = append(accounts, PassengerAccount{PassengerID: id, AccountID: "acc-" + strings.Replace(id, "pax-", "pax", 1)})
	}
	return CompensationCalculated{e.FlightID, amount * float64(len(e.EligibleIDs)), amount, "USD", e.EligibleIDs, accounts}
}
func ProjectDelayClaimSummary(d DelayEvaluated, e PassengerEligibilityVerified, c CompensationCalculated) DelayClaimSummary {
	passengers := make([]DelayClaimPassenger, 0, len(e.EligibleIDs))
	for _, id := range e.EligibleIDs {
		passengers = append(passengers, DelayClaimPassenger{id, c.PerPassengerAmount})
	}
	return DelayClaimSummary{d.FlightID, d.DelayMinutes, passengers, e.IneligibleIDs, c.TotalCompensationAmount, c.Currency, "pending_disbursement"}
}
func TranslateCompensationClosureProcessor(e FlightClosedOnTime) CompensationCaseClosed {
	return CompensationCaseClosed{e.FlightID, "on_time_no_claim", e.ClosedAt}
}
func DecideCloseCompensationCase(e FlightClosedOnTime, alreadyClosed bool) (string, *CompensationCaseClosed) {
	if alreadyClosed {
		return "CompensationCaseClosureRejected", nil
	}
	c := TranslateCompensationClosureProcessor(e)
	return "CompensationCaseClosed", &c
}
func ProjectClosedClaimRecord(e CompensationCaseClosed) string { return "closed_no_claim" }
func TranslateCompensationRecovery(e PartialDisbursementRejected) ClaimEscalated {
	return ClaimEscalated{e.FlightID, e.RejectedPassengerIDs, e.RejectedAmount, e.EscalatedAt}
}
func DecideEscalateUnresolvedClaim(e PartialDisbursementRejected, alreadyEscalated bool) (string, *ClaimEscalated) {
	if alreadyEscalated {
		return "ClaimEscalationRejected", nil
	}
	c := TranslateCompensationRecovery(e)
	return "ClaimEscalated", &c
}
func ProjectEscalatedClaimsQueue(e ClaimEscalated) ClaimEscalated { return e }

func TranslateFinance(c CompensationCalculated, approvedAt string) ApproveDisbursementCommand {
	return ApproveDisbursementCommand{c.FlightID, "disb-9901", c.TotalCompensationAmount, c.PerPassengerAmount, c.EligiblePassengerIDs, c.BillingAccounts, approvedAt}
}
func DecideApproveDisbursement(c ApproveDisbursementCommand) (string, *DisbursementApproved, *PartialDisbursementRejected) {
	covered := map[string]bool{}
	for _, a := range c.BillingAccounts {
		covered[a.PassengerID] = true
	}
	rejected := []string{}
	for _, id := range c.EligiblePassengerIDs {
		if !covered[id] {
			rejected = append(rejected, id)
		}
	}
	if len(rejected) > 0 {
		e := PartialDisbursementRejected{c.FlightID, rejected, c.PerPassengerAmount * float64(len(rejected)), "no_billable_account", "2024-06-16T10:00:00Z"}
		return "PartialDisbursementRejected", nil, &e
	}
	e := DisbursementApproved{c.FlightID, c.DisbursementID, c.TotalAmount, c.ApprovedAt, "Finance"}
	return "DisbursementApproved", &e, nil
}
func DecideIssuePayout(a DisbursementApproved, c CompensationCalculated, scheduled string) PayoutIssued {
	accounts := make([]string, 0, len(c.BillingAccounts))
	for _, account := range c.BillingAccounts {
		accounts = append(accounts, account.AccountID)
	}
	return PayoutIssued{a.DisbursementID, a.FlightID, "ACH", scheduled, a.TotalAmount, accounts, []string{"visitor-a", "visitor-b", "visitor-c"}, "delayed_flight_2024_06"}
}
func ProjectFinanceSettlementReport(e PayoutIssued) FinanceSettlementReport {
	rows := make([]AccountAmount, 0, len(e.BillingAccountIDs))
	each := e.TotalAmount / float64(len(e.BillingAccountIDs))
	for _, id := range e.BillingAccountIDs {
		rows = append(rows, AccountAmount{id, each})
	}
	return FinanceSettlementReport{e.DisbursementID, e.FlightID, e.TotalAmount, e.PaymentRails, e.ScheduledDate, "approved", rows}
}
func TranslatePayoutRecovery(e PayoutFailed) ReverseAndReissueCommand {
	return ReverseAndReissueCommand{e.DisbursementID, e.FailureCode, e.TotalAmount, e.FailedAt, e.RecoveryPaymentRails, e.RecoveryScheduledDate}
}
func DecideReverseAndReissue(c ReverseAndReissueCommand, alreadyReversed bool) PayoutRecoveryResult {
	if alreadyReversed {
		return PayoutRecoveryResult{Event: "PayoutReversalRejected", Reason: "already_reversed"}
	}
	reversed := PayoutReversed{c.DisbursementID, c.FailureCode, c.ReversedAt}
	reissued := PayoutReissued{c.DisbursementID, c.NewPaymentRails, c.ScheduledDate, c.TotalAmount}
	return PayoutRecoveryResult{Event: "PayoutReversedAndReissued", Reversed: &reversed, Reissued: &reissued}
}
func ProjectPayoutRecoveryReport(r PayoutRecoveryResult) (PayoutRecoveryReport, error) {
	if r.Reissued == nil {
		return PayoutRecoveryReport{}, fmt.Errorf("reissued payout required")
	}
	return PayoutRecoveryReport{r.Reissued.DisbursementID, "reissued_via_wire", r.Reissued.ScheduledDate}, nil
}

func TranslateMarketing(e PayoutIssued) TagDelayedTravelersCommand {
	return TagDelayedTravelersCommand{e.FlightID, e.TravelerIDs, e.SegmentTag, "booking_page_visitor"}
}
func DecideTagDelayedTravelers(c TagDelayedTravelersCommand) (string, *DelayedTravelerTagged) {
	if len(c.TravelerIDs) == 0 {
		return "TravelerTaggingSkipped", nil
	}
	e := DelayedTravelerTagged{c.FlightID, c.SegmentTag, len(c.TravelerIDs)}
	return "DelayedTravelerTagged", &e
}
func DecideSendRecoveryOffer(c TagDelayedTravelersCommand, offerID, channel, sentAt string) RecoveryOfferSent {
	return RecoveryOfferSent{c.SegmentTag, offerID, c.TravelerIDs, channel, len(c.TravelerIDs), sentAt}
}
func ProjectMarketingCampaignDashboard(tag DelayedTravelerTagged, sent RecoveryOfferSent, responses []RecoveryOfferResponse) MarketingCampaignDashboard {
	accepted, declined := 0, 0
	for _, response := range responses {
		if response.Event == "RecoveryOfferAccepted" {
			accepted++
		}
		if response.Event == "RecoveryOfferDeclined" {
			declined++
		}
	}
	rate := 0.0
	if sent.OffersSent > 0 {
		rate = float64(accepted) / float64(sent.OffersSent)
	}
	return MarketingCampaignDashboard{tag.SegmentTag, 1, tag.TravelerCount, sent.OffersSent, accepted, declined, rate}
}
func DecideRespondToRecoveryOffer(c RespondToRecoveryOfferCommand) (RecoveryOfferResponse, error) {
	switch c.Response {
	case "accepted":
		return RecoveryOfferResponse{"RecoveryOfferAccepted", c.OfferID, c.TravelerID, c.RespondedAt}, nil
	case "declined":
		return RecoveryOfferResponse{"RecoveryOfferDeclined", c.OfferID, c.TravelerID, c.RespondedAt}, nil
	default:
		return RecoveryOfferResponse{}, fmt.Errorf("response must be accepted or declined")
	}
}
func ProjectMarketingConversionReport(responses []RecoveryOfferResponse) MarketingConversionReport {
	r := MarketingConversionReport{}
	if len(responses) > 0 {
		r.OfferID = responses[0].OfferID
	}
	for _, response := range responses {
		if response.Event == "RecoveryOfferAccepted" {
			r.AcceptedCount++
		} else if response.Event == "RecoveryOfferDeclined" {
			r.DeclinedCount++
		}
	}
	return r
}
