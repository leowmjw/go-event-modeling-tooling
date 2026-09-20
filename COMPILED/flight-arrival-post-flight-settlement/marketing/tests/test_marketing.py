from extracted.marketing import *

def test_marketing_gwts_and_views():
    command = {"flightId": "HLT-421", "travelerIds": ["visitor-a", "visitor-b", "visitor-c"], "segmentTag": "delayed_flight_2024_06"}
    tagged = decide_tag_delayed_travelers(command)
    assert tagged["travelerCount"] == 3
    sent = decide_send_recovery_offer({**command, "offerId": "offer-77", "channel": "email", "sentAt": "2024-06-15T16:00:00Z"})
    accepted = decide_respond_to_recovery_offer({"offerId": "offer-77", "travelerId": "visitor-a", "response": "accepted", "respondedAt": "2024-06-15T17:05:00Z"})
    declined = decide_respond_to_recovery_offer({"offerId": "offer-77", "travelerId": "visitor-b", "response": "declined", "respondedAt": "2024-06-15T17:10:00Z"})
    dashboard = project_marketing_campaign_dashboard(tagged, sent, [accepted, declined])
    report = project_marketing_conversion_report([accepted, declined])
    assert dashboard["offersSent"] == 3 and dashboard["offersAccepted"] == 1 and dashboard["offersDeclined"] == 1
    assert report == {"offerId": "offer-77", "acceptedCount": 1, "declinedCount": 1}
