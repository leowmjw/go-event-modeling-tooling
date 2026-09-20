from extracted.operations import *

def test_operations_branches_and_views():
    command = {"flightId": "HLT-421", "wheelsTouchTime": "2024-06-15T14:32:00Z", "aircraftId": "N42HL"}
    assert decide_record_wheels_down(command, [])["event"] == "WheelsDownRecorded"
    assert decide_record_wheels_down(command, [{"event": "WheelsDownRecorded"}]) == {"event": "WheelsDownRejected", "flightId": "HLT-421", "reason": "already_recorded"}
    gate = decide_record_gate_arrival({"flightId": "HLT-421", "gateDoorOpenTime": "2024-06-15T14:47:00Z", "gateId": "B12"}, [{"event": "WheelsDownRecorded"}])
    assert project_arrival_board(gate) == {"flightId": "HLT-421", "gateId": "B12", "gateDoorOpenTime": "2024-06-15T14:47:00Z", "displayStatus": "ARRIVED"}
    closed = decide_close_flight_on_time({"flightId": "HLT-421", "closedAt": "2024-06-15T14:28:00Z"}, [{"event": "Operations.WheelsDownRecorded"}])
    assert project_flight_closure_record(closed) == {"flightId": "HLT-421", "status": "on_time", "closedAt": "2024-06-15T14:28:00Z"}
