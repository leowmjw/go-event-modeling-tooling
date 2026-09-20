from extracted.conversation_agent import decide_ask_question, decide_record_answer, decide_request_invoice_search, format_invoice_answer, project_answers


def test_complete_success_payloads():
    asked = decide_ask_question({"conversationId": "conv-1001", "question": "How many invoices were sent out yesterday?", "requestedAt": "2026-09-20T13:00:00Z", "businessTimezone": "America/New_York"})
    assert asked["event"] == "QuestionAsked"
    request = decide_request_invoice_search({"conversationId": "conv-1001", "fromDate": "2026-09-19", "toDate": "2026-09-19"})
    assert request == {"event": "Conversation.InvoiceSearchRequested", "conversationId": "conv-1001", "fromDate": "2026-09-19", "toDate": "2026-09-19"}
    command = format_invoice_answer({"conversationId": "conv-1001", "count": 2, "customers": ["X", "Y"]})
    assert command == {"conversationId": "conv-1001", "answer": "2 invoices were sent out yesterday to customers X and Y."}
    recorded = decide_record_answer(command)
    assert project_answers(recorded) == command
