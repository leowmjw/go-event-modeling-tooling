from __future__ import annotations


def project_flight_status(event: dict) -> dict:
    return {key: event[key] for key in ("flightId", "origin", "destination", "scheduledArrival", "currentStatus", "aircraftId")}


def decide_record_wheels_down(command: dict, prior_events: list[dict]) -> dict:
    if any(event.get("event") == "WheelsDownRecorded" for event in prior_events):
        return {"event": "WheelsDownRejected", "flightId": command["flightId"], "reason": "already_recorded"}
    return {"event": "WheelsDownRecorded", **command, "boundedContext": "Operations"}


def decide_record_gate_arrival(command: dict, prior_events: list[dict]) -> dict:
    if not any(event.get("event") == "WheelsDownRecorded" for event in prior_events):
        return {"event": "GateArrivalRejected", "flightId": command["flightId"], "reason": "wheels_down_not_recorded"}
    return {"event": "GateOpened", **command, "boundedContext": "Operations"}


def project_arrival_board(event: dict) -> dict:
    return {"flightId": event["flightId"], "gateId": event["gateId"], "gateDoorOpenTime": event["gateDoorOpenTime"], "displayStatus": "ARRIVED"}


def translate_ontime_verifier(input: dict) -> dict:
    return {"flightId": input["flightId"], "closedAt": input["wheelsTouchTime"], "reason": "on_time_wheels_down"}


def decide_close_flight_on_time(command: dict, prior_events: list[dict]) -> dict:
    if not any(event.get("event", "").endswith("WheelsDownRecorded") for event in prior_events):
        return {"event": "FlightClosureRejected", "flightId": command["flightId"], "reason": "wheels_down_not_recorded"}
    return {"event": "FlightClosedOnTime", "flightId": command["flightId"], "closedAt": command["closedAt"]}


def project_flight_closure_record(event: dict) -> dict:
    return {"flightId": event["flightId"], "status": "on_time", "closedAt": event["closedAt"]}
