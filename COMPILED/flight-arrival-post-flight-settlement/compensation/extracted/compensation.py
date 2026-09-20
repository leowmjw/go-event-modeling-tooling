from __future__ import annotations
from datetime import datetime


def translate_compensation_processor(input: dict) -> dict:
    return {"flightId": input["flightId"], "scheduledArrival": input["scheduledArrival"], "actualArrivalTime": input["gateDoorOpenTime"], "onTimeDefinition": "gate_door_open", "onTimeThresholdMinutes": input["onTimeThresholdMinutes"]}


def decide_evaluate_delay(command: dict) -> dict:
    scheduled = datetime.fromisoformat(command["scheduledArrival"].replace("Z", "+00:00"))
    actual = datetime.fromisoformat(command["actualArrivalTime"].replace("Z", "+00:00"))
    minutes = int((actual - scheduled).total_seconds() / 60)
    return {"event": "DelayEvaluated", "flightId": command["flightId"], "delayMinutes": minutes, "isDelayed": minutes > command["onTimeThresholdMinutes"], "onTimeThresholdMinutes": command["onTimeThresholdMinutes"], "boundedContext": "Compensation"}


def decide_verify_passenger_eligibility(command: dict, prior_events: list[dict]) -> dict:
    paid = {pid for event in prior_events if event.get("event") == "CompensationCalculated" for pid in event.get("eligiblePassengerIds", [])}
    eligible = [pid for pid in command["passengerIds"] if pid not in paid]
    return {"event": "PassengerEligibilityVerified", "flightId": command["flightId"], "eligibleIds": eligible, "ineligibleIds": [pid for pid in command["passengerIds"] if pid in paid]}


def decide_calculate_compensation(command: dict) -> dict:
    amount = 170.0
    ids = command["eligiblePassengerIds"]
    return {"event": "CompensationCalculated", "flightId": command["flightId"], "totalCompensationAmount": amount * len(ids), "perPassengerAmount": amount, "currency": "USD", "eligiblePassengerIds": ids}


def project_delay_claim_summary(delay: dict, eligibility: dict, compensation: dict) -> dict:
    amount = compensation["perPassengerAmount"]
    return {"flightId": delay["flightId"], "delayMinutes": delay["delayMinutes"], "eligiblePassengers": [{"passengerId": pid, "compensationAmount": amount} for pid in eligibility["eligibleIds"]], "ineligiblePassengers": eligibility["ineligibleIds"], "totalCompensation": compensation["totalCompensationAmount"], "currency": compensation["currency"], "status": "pending_disbursement"}


def translate_compensation_closure_processor(input: dict) -> dict:
    return {"flightId": input["flightId"], "reason": "on_time_no_claim", "closedAt": input["closedAt"]}


def decide_close_compensation_case(command: dict, prior_events: list[dict]) -> dict:
    if any(event.get("event") == "CompensationCaseClosed" for event in prior_events):
        return {"event": "CompensationCaseClosureRejected", "flightId": command["flightId"], "reason": "already_closed"}
    return {"event": "CompensationCaseClosed", **command}


def project_closed_claim_record(event: dict) -> dict:
    return {"flightId": event["flightId"], "status": "closed_no_claim"}


def translate_compensation_recovery(input: dict) -> dict:
    return {"flightId": input["flightId"], "passengerIds": input["rejectedPassengerIds"], "amount": input["rejectedAmount"], "escalatedAt": input["escalatedAt"]}


def decide_escalate_unresolved_claim(command: dict, prior_events: list[dict]) -> dict:
    if any(event.get("event") == "ClaimEscalated" for event in prior_events):
        return {"event": "ClaimEscalationRejected", "flightId": command["flightId"], "reason": "already_escalated"}
    return {"event": "ClaimEscalated", **command}


def project_escalated_claims_queue(event: dict) -> dict:
    return {"flightId": event["flightId"], "pendingPassengers": event["passengerIds"], "pendingAmount": event["amount"]}
