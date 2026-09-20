from __future__ import annotations


def decide_add_item(command: dict, prior_events: list[dict]) -> dict:
    if command["price"] <= 0:
        return {"event": "ItemAddRejected", "item_add_rejected": {"itemId": command["itemId"], "reason": "price_must_be_positive"}}
    item = {key: command[key] for key in ("itemId", "description", "image", "price")}
    return {"event": "ItemAdded", "item_added": item}


def project_item_catalog(result: dict) -> dict:
    item = result.get("item_added")
    return {"items": [item] if item else []}
