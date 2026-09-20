package settlement

import (
	"fmt"

	"go.temporal.io/sdk/workflow"
)

func OperationsWorkflow(_ workflow.Context, input OperationsInput) (OperationsResult, error) {
	ProjectFlightStatus(input.Flight)
	event, wheels := DecideRecordWheelsDown(input.WheelsDown, false)
	if wheels == nil {
		return OperationsResult{Event: event}, nil
	}
	result := OperationsResult{Event: event, WheelsDown: *wheels}
	if input.OnTime {
		closed := DecideCloseFlightOnTime(*wheels)
		record := ProjectFlightClosureRecord(closed)
		result.Event, result.ClosedOnTime, result.ClosureRecord = "FlightClosedOnTime", &closed, &record
		return result, nil
	}
	event, gate := DecideRecordGateArrival(input.GateArrival, true, input.Flight.ScheduledArrival)
	result.Event = event
	if gate != nil {
		board := ProjectArrivalBoard(*gate)
		result.GateOpened, result.ArrivalBoard = gate, &board
	}
	return result, nil
}

func CompensationWorkflow(_ workflow.Context, input CompensationInput) (CompensationResult, error) {
	if input.ClosedOnTime != nil {
		event, closed := DecideCloseCompensationCase(*input.ClosedOnTime, false)
		return CompensationResult{Event: event, Closed: closed}, nil
	}
	if input.PartialRejected != nil {
		event, escalated := DecideEscalateUnresolvedClaim(*input.PartialRejected, false)
		if escalated != nil {
			ProjectEscalatedClaimsQueue(*escalated)
		}
		return CompensationResult{Event: event, Escalated: escalated}, nil
	}
	if input.GateOpened == nil {
		return CompensationResult{}, fmt.Errorf("a compensation input event is required")
	}
	command := TranslateCompensationProcessor(*input.GateOpened)
	delay, err := DecideEvaluateDelay(command)
	if err != nil {
		return CompensationResult{}, err
	}
	if !delay.IsDelayed {
		return CompensationResult{Event: "DelayEvaluationSuppressed", Delay: &delay}, nil
	}
	eligibility := DecideVerifyPassengerEligibility(delay.FlightID, input.PassengerIDs, nil)
	calculated := DecideCalculateCompensation(eligibility)
	summary := ProjectDelayClaimSummary(delay, eligibility, calculated)
	return CompensationResult{Event: "CompensationCalculated", Delay: &delay, Eligibility: &eligibility, Calculated: &calculated, Summary: &summary}, nil
}

func FinanceWorkflow(_ workflow.Context, input FinanceInput) (FinanceResult, error) {
	command := TranslateFinance(input.Compensation, input.ApprovedAt)
	event, approved, partial := DecideApproveDisbursement(command)
	result := FinanceResult{Event: event, Approved: approved, PartialRejected: partial}
	if approved == nil {
		return result, nil
	}
	payout := DecideIssuePayout(*approved, input.Compensation, input.ScheduledDate)
	report := ProjectFinanceSettlementReport(payout)
	result.Event, result.Payout, result.Report = "PayoutIssued", &payout, &report
	return result, nil
}

func MarketingWorkflow(ctx workflow.Context, input MarketingInput) (MarketingResult, error) {
	command := TranslateMarketing(input.Payout)
	event, tagged := DecideTagDelayedTravelers(command)
	if tagged == nil {
		return MarketingResult{Event: event}, nil
	}
	sent := DecideSendRecoveryOffer(command, input.OfferID, input.Channel, input.SentAt)
	responses := []RecoveryOfferResponse{}
	dashboard := ProjectMarketingCampaignDashboard(*tagged, sent, responses)
	conversion := ProjectMarketingConversionReport(responses)
	if err := workflow.SetQueryHandler(ctx, CampaignDashboardQuery, func() (MarketingCampaignDashboard, error) { return dashboard, nil }); err != nil {
		return MarketingResult{}, err
	}
	if err := workflow.SetQueryHandler(ctx, ConversionReportQuery, func() (MarketingConversionReport, error) { return conversion, nil }); err != nil {
		return MarketingResult{}, err
	}
	var responseCommand RespondToRecoveryOfferCommand
	workflow.GetSignalChannel(ctx, RecoveryResponseSignal).Receive(ctx, &responseCommand)
	response, err := DecideRespondToRecoveryOffer(responseCommand)
	if err != nil {
		return MarketingResult{}, err
	}
	responses = append(responses, response)
	dashboard = ProjectMarketingCampaignDashboard(*tagged, sent, responses)
	conversion = ProjectMarketingConversionReport(responses)
	return MarketingResult{Event: response.Event, Tagged: *tagged, Sent: sent, Response: response, Dashboard: dashboard, Conversion: conversion}, nil
}

func ParentWorkflow(ctx workflow.Context, input ParentInput) (ParentResult, error) {
	workflowID := workflow.GetInfo(ctx).WorkflowExecution.ID
	var operations OperationsResult
	if err := executeChild(ctx, workflowID+"/operations", OperationsWorkflowName, input.Operations, &operations); err != nil {
		return ParentResult{}, err
	}
	result := ParentResult{Operations: operations}
	if operations.ClosedOnTime != nil {
		var compensation CompensationResult
		if err := executeChild(ctx, workflowID+"/compensation-close", CompensationWorkflowName, CompensationInput{ClosedOnTime: operations.ClosedOnTime}, &compensation); err != nil {
			return ParentResult{}, err
		}
		result.Compensation = compensation
		return result, nil
	}
	if operations.GateOpened == nil {
		return ParentResult{}, fmt.Errorf("operations produced neither gate arrival nor on-time closure")
	}
	var compensation CompensationResult
	if err := executeChild(ctx, workflowID+"/compensation", CompensationWorkflowName, CompensationInput{GateOpened: operations.GateOpened, PassengerIDs: input.PassengerIDs}, &compensation); err != nil {
		return ParentResult{}, err
	}
	result.Compensation = compensation
	if compensation.Calculated == nil {
		return result, nil
	}
	var finance FinanceResult
	if err := executeChild(ctx, workflowID+"/finance", FinanceWorkflowName, FinanceInput{Compensation: *compensation.Calculated, ApprovedAt: input.ApprovedAt, ScheduledDate: input.ScheduledDate}, &finance); err != nil {
		return ParentResult{}, err
	}
	result.Finance = &finance
	if finance.PartialRejected != nil {
		var recovery CompensationResult
		if err := executeChild(ctx, workflowID+"/compensation-recovery", CompensationWorkflowName, CompensationInput{PartialRejected: finance.PartialRejected}, &recovery); err != nil {
			return ParentResult{}, err
		}
		result.Compensation.Escalated = recovery.Escalated
	}
	if finance.Payout == nil {
		return result, nil
	}
	var marketing MarketingResult
	if err := executeChild(ctx, workflowID+"/marketing", MarketingWorkflowName, MarketingInput{Payout: *finance.Payout, OfferID: input.OfferID, Channel: input.Channel, SentAt: input.SentAt}, &marketing); err != nil {
		return ParentResult{}, err
	}
	result.Marketing = &marketing
	return result, nil
}

func executeChild(ctx workflow.Context, id, name string, input, output any) error {
	child := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{WorkflowID: id})
	return workflow.ExecuteChildWorkflow(child, name, input).Get(child, output)
}
