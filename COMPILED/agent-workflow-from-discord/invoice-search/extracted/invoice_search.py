from __future__ import annotations


def invoice_repository(request: dict) -> dict:
    return {"conversationId": request["conversationId"], "count": 2, "customers": ["X", "Y"], "failureReason": ""}


def decide_record_invoice_search_result(command: dict) -> dict:
    if command.get("failureReason"):
        return {"event": "InvoiceSearch.InvoiceSearchFailed", "conversationId": command["conversationId"], "reason": command["failureReason"]}
    return {"event": "InvoiceSearch.InvoicesFound", "conversationId": command["conversationId"], "count": int(command["count"]), "customers": list(command["customers"])}


def project_invoice_search_result(event: dict) -> dict:
    if event["event"] == "InvoiceSearch.InvoiceSearchFailed":
        return {"conversationId": event["conversationId"], "status": "failed", "reason": event["reason"]}
    return {"conversationId": event["conversationId"], "status": "found", "count": event["count"], "customers": event["customers"]}
