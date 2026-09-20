from extracted.invoice_search import decide_record_invoice_search_result, project_invoice_search_result


def test_success_and_failure_payloads():
    success = decide_record_invoice_search_result({"conversationId": "conv-1001", "count": 2, "customers": ["X", "Y"], "failureReason": ""})
    assert success == {"event": "InvoiceSearch.InvoicesFound", "conversationId": "conv-1001", "count": 2, "customers": ["X", "Y"]}
    assert project_invoice_search_result(success)["status"] == "found"
    failure = decide_record_invoice_search_result({"conversationId": "conv-1001", "count": 0, "customers": [], "failureReason": "repository_unavailable"})
    assert failure == {"event": "InvoiceSearch.InvoiceSearchFailed", "conversationId": "conv-1001", "reason": "repository_unavailable"}
    assert project_invoice_search_result(failure) == {"conversationId": "conv-1001", "status": "failed", "reason": "repository_unavailable"}
