package settlement

import (
	"testing"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestOperationsGWTBranches(t *testing.T) {
	command := RecordWheelsDownCommand{FlightID: "HLT-421", WheelsTouchTime: "2024-06-15T14:32:00Z", AircraftID: "N42HL"}
	if event, result := DecideRecordWheelsDown(command, false); event != "WheelsDownRecorded" || result == nil || result.BoundedContext != "Operations" {
		t.Fatalf("unexpected wheels-down: %s %#v", event, result)
	}
	if event, result := DecideRecordWheelsDown(command, true); event != "WheelsDownRejected" || result != nil {
		t.Fatalf("duplicate not rejected")
	}
	gate := RecordGateArrivalCommand{FlightID: "HLT-421", GateDoorOpenTime: "2024-06-15T14:47:00Z", GateID: "B12"}
	if event, result := DecideRecordGateArrival(gate, true, "2024-06-15T14:30:00Z"); event != "GateOpened" || result == nil || result.GateID != "B12" {
		t.Fatalf("unexpected gate: %s %#v", event, result)
	}
	if event, result := DecideRecordGateArrival(gate, false, "2024-06-15T14:30:00Z"); event != "GateArrivalRejected" || result != nil {
		t.Fatalf("gate without wheels-down not rejected")
	}
}

func TestCompensationGWTBranchesAndProjection(t *testing.T) {
	delayed, err := DecideEvaluateDelay(EvaluateDelayCommand{"HLT-421", "2024-06-15T14:30:00Z", "2024-06-15T14:47:00Z", "gate_door_open", 15})
	if err != nil || delayed.DelayMinutes != 17 || !delayed.IsDelayed {
		t.Fatalf("delay = %#v, err = %v", delayed, err)
	}
	onTime, err := DecideEvaluateDelay(EvaluateDelayCommand{"HLT-421", "2024-06-15T14:30:00Z", "2024-06-15T14:44:00Z", "gate_door_open", 15})
	if err != nil || onTime.DelayMinutes != 14 || onTime.IsDelayed {
		t.Fatalf("on-time = %#v, err = %v", onTime, err)
	}
	eligibility := DecideVerifyPassengerEligibility("HLT-421", []string{"pax-1", "pax-2", "pax-3"}, []string{"pax-3"})
	if len(eligibility.EligibleIDs) != 2 || len(eligibility.IneligibleIDs) != 1 {
		t.Fatalf("eligibility = %#v", eligibility)
	}
	calculated := DecideCalculateCompensation(PassengerEligibilityVerified{FlightID: "HLT-421", EligibleIDs: []string{"pax-1", "pax-2", "pax-3"}, IneligibleIDs: []string{}})
	summary := ProjectDelayClaimSummary(delayed, PassengerEligibilityVerified{FlightID: "HLT-421", EligibleIDs: calculated.EligiblePassengerIDs, IneligibleIDs: []string{}}, calculated)
	if calculated.TotalCompensationAmount != 510 || calculated.Currency != "USD" || summary.TotalCompensation != 510 || len(summary.EligiblePassengers) != 3 {
		t.Fatalf("projection = %#v %#v", calculated, summary)
	}
}

func TestFinanceAndRecoveryGWTBranches(t *testing.T) {
	calculated := CompensationCalculated{FlightID: "HLT-421", TotalCompensationAmount: 510, PerPassengerAmount: 170, EligiblePassengerIDs: []string{"pax-1", "pax-2", "pax-3"}, BillingAccounts: []PassengerAccount{{"pax-1", "acc-pax1"}, {"pax-2", "acc-pax2"}, {"pax-3", "acc-pax3"}}}
	command := TranslateFinance(calculated, "2024-06-15T15:10:00Z")
	event, approved, rejected := DecideApproveDisbursement(command)
	if event != "DisbursementApproved" || approved == nil || rejected != nil {
		t.Fatalf("approval = %s %#v %#v", event, approved, rejected)
	}
	command.BillingAccounts = command.BillingAccounts[:2]
	event, _, rejected = DecideApproveDisbursement(command)
	if event != "PartialDisbursementRejected" || rejected == nil || rejected.RejectedAmount != 170 {
		t.Fatalf("partial = %s %#v", event, rejected)
	}
	recovery := DecideReverseAndReissue(ReverseAndReissueCommand{"disb-9901", "R03", 510, "2024-06-16T09:00:00Z", "WIRE", "2024-06-17"}, false)
	if recovery.Reversed == nil || recovery.Reissued == nil || recovery.Reissued.TotalAmount != 510 {
		t.Fatalf("recovery = %#v", recovery)
	}
	if DecideReverseAndReissue(ReverseAndReissueCommand{DisbursementID: "disb-9901"}, true).Event != "PayoutReversalRejected" {
		t.Fatal("duplicate reversal not rejected")
	}
}

func TestMarketingGWTBranches(t *testing.T) {
	command := TagDelayedTravelersCommand{FlightID: "HLT-421", TravelerIDs: []string{"visitor-a", "visitor-b", "visitor-c"}, SegmentTag: "delayed_flight_2024_06"}
	event, tagged := DecideTagDelayedTravelers(command)
	if event != "DelayedTravelerTagged" || tagged.TravelerCount != 3 {
		t.Fatalf("tagging = %s %#v", event, tagged)
	}
	if event, tagged := DecideTagDelayedTravelers(TagDelayedTravelersCommand{FlightID: "HLT-421"}); event != "TravelerTaggingSkipped" || tagged != nil {
		t.Fatal("empty audience not skipped")
	}
	sent := DecideSendRecoveryOffer(command, "offer-77", "email", "2024-06-15T16:00:00Z")
	accepted, err := DecideRespondToRecoveryOffer(RespondToRecoveryOfferCommand{"offer-77", "visitor-a", "accepted", "2024-06-15T17:05:00Z"})
	if err != nil || accepted.Event != "RecoveryOfferAccepted" {
		t.Fatalf("accepted = %#v %v", accepted, err)
	}
	declined, err := DecideRespondToRecoveryOffer(RespondToRecoveryOfferCommand{"offer-77", "visitor-b", "declined", "2024-06-15T17:10:00Z"})
	if err != nil || declined.Event != "RecoveryOfferDeclined" {
		t.Fatalf("declined = %#v %v", declined, err)
	}
	dashboard := ProjectMarketingCampaignDashboard(*tagged, sent, []RecoveryOfferResponse{accepted, declined})
	conversion := ProjectMarketingConversionReport([]RecoveryOfferResponse{accepted, declined})
	if dashboard.OffersAccepted != 1 || dashboard.OffersDeclined != 1 || conversion.AcceptedCount != 1 || conversion.DeclinedCount != 1 {
		t.Fatalf("views = %#v %#v", dashboard, conversion)
	}
}

func TestParentDelayedCrossContextPath(t *testing.T) {
	env := newParentEnvironment()
	env.SetStartWorkflowOptions(client.StartWorkflowOptions{ID: "flight-HLT-421"})
	env.RegisterDelayedCallback(func() {
		if err := env.SignalWorkflowByID("flight-HLT-421/marketing", RecoveryResponseSignal, RespondToRecoveryOfferCommand{"offer-77", "visitor-a", "accepted", "2024-06-15T17:05:00Z"}); err != nil {
			t.Fatal(err)
		}
	}, time.Second)
	env.ExecuteWorkflow(ParentWorkflow, parentInput(false))
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result ParentResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Compensation.Calculated == nil || result.Finance == nil || result.Finance.Payout == nil || result.Marketing == nil || result.Marketing.Response.Event != "RecoveryOfferAccepted" {
		t.Fatalf("result = %#v", result)
	}
}

func TestParentOnTimeCrossContextPath(t *testing.T) {
	env := newParentEnvironment()
	input := parentInput(true)
	env.ExecuteWorkflow(ParentWorkflow, input)
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result ParentResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Operations.ClosedOnTime == nil || result.Compensation.Closed == nil || result.Finance != nil {
		t.Fatalf("result = %#v", result)
	}
}

func newParentEnvironment() *testsuite.TestWorkflowEnvironment {
	env := new(testsuite.WorkflowTestSuite).NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(OperationsWorkflow, workflow.RegisterOptions{Name: OperationsWorkflowName})
	env.RegisterWorkflowWithOptions(CompensationWorkflow, workflow.RegisterOptions{Name: CompensationWorkflowName})
	env.RegisterWorkflowWithOptions(FinanceWorkflow, workflow.RegisterOptions{Name: FinanceWorkflowName})
	env.RegisterWorkflowWithOptions(MarketingWorkflow, workflow.RegisterOptions{Name: MarketingWorkflowName})
	return env
}
func parentInput(onTime bool) ParentInput {
	return ParentInput{Operations: OperationsInput{Flight: Flight{"HLT-421", "JFK", "LAX", "2024-06-15T14:30:00Z", "in_flight", "N42HL"}, WheelsDown: RecordWheelsDownCommand{"HLT-421", map[bool]string{true: "2024-06-15T14:28:00Z", false: "2024-06-15T14:32:00Z"}[onTime], "N42HL"}, GateArrival: RecordGateArrivalCommand{"HLT-421", "2024-06-15T14:47:00Z", "B12"}, OnTime: onTime}, PassengerIDs: []string{"pax-1", "pax-2", "pax-3"}, ApprovedAt: "2024-06-15T15:10:00Z", ScheduledDate: "2024-06-16", OfferID: "offer-77", Channel: "email", SentAt: "2024-06-15T16:00:00Z"}
}
