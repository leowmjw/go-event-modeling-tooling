from extracted.support_operations import project_support_review_inbox


def test_projection_matches_data_block():
    assert project_support_review_inbox({"conversationId": "conv-1001", "reason": "repository_unavailable"}) == {"items": [{"conversationId": "conv-1001", "reason": "repository_unavailable"}]}
