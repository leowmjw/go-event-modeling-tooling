from __future__ import annotations

from pydantic import BaseModel


class SelectInvoiceToolJudgeInput(BaseModel):
    conversationId: str
    question: str
    requestedAt: str
    businessTimezone: str


class SelectInvoiceToolJudgeOutput(BaseModel):
    conversationId: str
    tool: str
    fromDate: str
    toDate: str


class SelectInvoiceToolJudge:
    def forward(self, value: SelectInvoiceToolJudgeInput) -> SelectInvoiceToolJudgeOutput:
        if value.businessTimezone != "America/New_York" or value.requestedAt != "2026-09-20T13:00:00Z":
            raise ValueError("unsupported crystallized demo date contract")
        return SelectInvoiceToolJudgeOutput(conversationId=value.conversationId, tool="findInvoices", fromDate="2026-09-19", toDate="2026-09-19")
