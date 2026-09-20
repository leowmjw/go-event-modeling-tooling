from __future__ import annotations


def decide_ask_question(command: dict) -> dict:
    return {"event": "QuestionAsked", **command}


def decide_request_invoice_search(command: dict) -> dict:
    return {"event": "Conversation.InvoiceSearchRequested", "conversationId": command["conversationId"], "fromDate": command["fromDate"], "toDate": command["toDate"]}


def format_invoice_answer(event: dict) -> dict:
    count = int(event["count"])
    noun, verb = ("invoice", "was") if count == 1 else ("invoices", "were")
    customers = list(event["customers"])
    if len(customers) > 1:
        names = ", ".join(customers[:-1]) + " and " + customers[-1]
    else:
        names = customers[0] if customers else "no customers"
    return {"conversationId": event["conversationId"], "answer": f"{count} {noun} {verb} sent out yesterday to customers {names}."}


def decide_record_answer(command: dict) -> dict:
    return {"event": "AnswerRecorded", **command}


def project_answers(event: dict) -> dict:
    return {"conversationId": event["conversationId"], "answer": event["answer"]}
