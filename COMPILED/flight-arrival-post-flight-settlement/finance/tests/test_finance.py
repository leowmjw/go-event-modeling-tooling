from extracted.finance import *

def test_finance_gwts_and_projections():
    command = {"flightId": "HLT-421", "disbursementId": "disb-9901", "totalAmount": 510, "perPassengerAmount": 170, "eligiblePassengerIds": ["pax-1", "pax-2", "pax-3"], "billingAccounts": [{"passengerId": "pax-1", "accountId": "acc-pax1"}, {"passengerId": "pax-2", "accountId": "acc-pax2"}, {"passengerId": "pax-3", "accountId": "acc-pax3"}], "approvedAt": "2024-06-15T15:10:00Z"}
    assert decide_approve_disbursement(command)["event"] == "DisbursementApproved"
    command["billingAccounts"] = command["billingAccounts"][:2]
    partial = decide_approve_disbursement(command)
    assert partial["event"] == "PartialDisbursementRejected" and partial["rejectedAmount"] == 170
    recovery = decide_reverse_and_reissue({"disbursementId": "disb-9901", "failureCode": "R03", "totalAmount": 510, "reversedAt": "2024-06-16T09:00:00Z", "newPaymentRails": "WIRE", "scheduledDate": "2024-06-17"}, [])
    assert recovery["payoutReversed"]["failureCode"] == "R03" and recovery["payoutReissued"]["totalAmount"] == 510
    assert project_payout_recovery_report(recovery)["status"] == "reissued_via_wire"
