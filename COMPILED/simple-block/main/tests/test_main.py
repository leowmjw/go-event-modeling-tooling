from extracted.main import decide_add_item, project_item_catalog


def test_add_valid_item() -> None:
    command = {"itemId": "item-1", "description": "john", "image": "avatar_john", "price": 20.4}
    result = decide_add_item(command, [{"event": "ItemCatalogOpened", "catalogId": "catalog-1"}])
    assert result == {"event": "ItemAdded", "item_added": command}
    assert project_item_catalog(result) == {"items": [command]}


def test_reject_non_positive_price() -> None:
    command = {"itemId": "item-2", "description": "jane", "image": "avatar_jane", "price": 0}
    assert decide_add_item(command, []) == {
        "event": "ItemAddRejected",
        "item_add_rejected": {"itemId": "item-2", "reason": "price_must_be_positive"},
    }
