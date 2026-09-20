from extracted.compensation import *

def test_compensation_gwts_and_projection():
    delayed = decide_evaluate_delay({"flightId": "HLT-421", "scheduledArrival": "2024-06-15T14:30:00Z", "actualArrivalTime": "2024-06-15T14:47:00Z", "onTimeThresholdMinutes": 15})
    assert delayed["delayMinutes"] == 17 and delayed["isDelayed"] is True
    eligible = decide_verify_passenger_eligibility({"flightId": "HLT-421", "passengerIds": ["pax-1", "pax-2", "pax-3"]}, [{"event": "CompensationCalculated", "eligiblePassengerIds": ["pax-3"]}])
    assert eligible["eligibleIds"] == ["pax-1", "pax-2"] and eligible["ineligibleIds"] == ["pax-3"]
    all_eligible = {"event": "PassengerEligibilityVerified", "flightId": "HLT-421", "eligibleIds": ["pax-1", "pax-2", "pax-3"], "ineligibleIds": []}
    calculated = decide_calculate_compensation({"flightId": "HLT-421", "eligiblePassengerIds": all_eligible["eligibleIds"]})
    summary = project_delay_claim_summary(delayed, all_eligible, calculated)
    assert calculated["totalCompensationAmount"] == 510 and summary["totalCompensation"] == 510 and len(summary["eligiblePassengers"]) == 3
