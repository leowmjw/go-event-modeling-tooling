from __future__ import annotations


def project_support_review_inbox(event: dict) -> dict:
    return {"items": [{"conversationId": event["conversationId"], "reason": event["reason"]}]}
