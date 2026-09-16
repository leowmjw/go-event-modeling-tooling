from __future__ import annotations

from pydantic import BaseModel, Field


class Agent_AJudge(BaseModel):
    """LLM judge stub for Agent_A."""
    input: dict = Field(default_factory=dict)
    output: dict = Field(default_factory=dict)
