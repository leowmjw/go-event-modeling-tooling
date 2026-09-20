from __future__ import annotations


def translate_finance(input: dict) -> dict:
    return {"flightId": input["flightId"], "disbursementId": "disb-9901", "totalAmount": input["totalCompensationAmount"], "perPassengerAmount": input["perPassengerAmount"], "eligiblePassengerIds": input["eligiblePassengerIds"], "billingAccounts": input["billingAccounts"], "customerDefinition": "billable_account", "approvedAt": "2024-06-15T15:10:00Z"}


def decide_approve_disbursement(command: dict) -> dict:
    covered = {row["passengerId"] for row in command["billingAccounts"]}
    rejected = [pid for pid in command["eligiblePassengerIds"] if pid not in covered]
    if rejected:
        return {"event": "PartialDisbursementRejected" if covered else "DisbursementRejected", "flightId": command["flightId"], "rejectedPassengerIds": rejected, "rejectedAmount": command["perPassengerAmount"] * len(rejected), "reason": "no_billable_account"}
    return {"event": "DisbursementApproved", "flightId": command["flightId"], "disbursementId": command["disbursementId"], "totalAmount": command["totalAmount"], "approvedAt": command["approvedAt"], "boundedContext": "Finance"}


def decide_issue_payout(command: dict, prior_events: list[dict]) -> dict:
    if not any(event.get("event") == "DisbursementApproved" for event in prior_events):
        return {"event": "PayoutRejected", "disbursementId": command["disbursementId"], "reason": "disbursement_not_approved"}
    return {"event": "PayoutIssued", **command}


def project_finance_settlement_report(event: dict) -> dict:
    each = event["totalAmount"] / len(event["billingAccountIds"])
    return {"disbursementId": event["disbursementId"], "flightId": event["flightId"], "totalAmount": event["totalAmount"], "paymentRails": event["paymentRails"], "scheduledDate": event["scheduledDate"], "status": "approved", "billingAccounts": [{"accountId": account, "amount": each} for account in event["billingAccountIds"]]}


def translate_payout_recovery(input: dict) -> dict:
    return {"disbursementId": input["disbursementId"], "failureCode": input["failureCode"], "totalAmount": input["totalAmount"], "reversedAt": input["failedAt"], "newPaymentRails": input["recoveryPaymentRails"], "scheduledDate": input["recoveryScheduledDate"]}


def decide_reverse_and_reissue(command: dict, prior_events: list[dict]) -> dict:
    if any(event.get("event") == "PayoutReversed" for event in prior_events):
        return {"event": "PayoutReversalRejected", "disbursementId": command["disbursementId"], "reason": "already_reversed"}
    return {"event": "PayoutReversedAndReissued", "payoutReversed": {"disbursementId": command["disbursementId"], "failureCode": command["failureCode"], "reversedAt": command["reversedAt"]}, "payoutReissued": {"disbursementId": command["disbursementId"], "newPaymentRails": command["newPaymentRails"], "scheduledDate": command["scheduledDate"], "totalAmount": command["totalAmount"]}}


def project_payout_recovery_report(result: dict) -> dict:
    event = result["payoutReissued"]
    return {"disbursementId": event["disbursementId"], "status": "reissued_via_wire", "scheduledDate": event["scheduledDate"]}
