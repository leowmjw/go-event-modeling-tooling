from __future__ import annotations


def translate_marketing(input: dict) -> dict:
    return {"flightId": input["flightId"], "travelerIds": input["travelerIds"], "segmentTag": input["segmentTag"], "customerDefinition": "booking_page_visitor"}


def decide_tag_delayed_travelers(command: dict) -> dict:
    if not command["travelerIds"]:
        return {"event": "TravelerTaggingSkipped", "flightId": command["flightId"], "reason": "no_matching_visitors"}
    return {"event": "DelayedTravelerTagged", "flightId": command["flightId"], "segmentTag": command["segmentTag"], "travelerCount": len(command["travelerIds"])}


def decide_send_recovery_offer(command: dict) -> dict:
    return {"event": "RecoveryOfferSent", "segmentTag": command["segmentTag"], "offerId": command["offerId"], "travelerIds": command["travelerIds"], "channel": command["channel"], "offersSent": len(command["travelerIds"]), "sentAt": command["sentAt"]}


def project_marketing_campaign_dashboard(tagged: dict, sent: dict, responses: list[dict]) -> dict:
    accepted = sum(event.get("event") == "RecoveryOfferAccepted" for event in responses)
    declined = sum(event.get("event") == "RecoveryOfferDeclined" for event in responses)
    return {"segmentTag": tagged["segmentTag"], "activeCampaigns": 1, "targetedTravelers": tagged["travelerCount"], "offersSent": sent["offersSent"], "offersAccepted": accepted, "offersDeclined": declined, "conversionRate": accepted / sent["offersSent"] if sent["offersSent"] else 0.0}


def decide_respond_to_recovery_offer(command: dict) -> dict:
    if command["response"] == "accepted":
        return {"event": "RecoveryOfferAccepted", "offerId": command["offerId"], "travelerId": command["travelerId"], "acceptedAt": command["respondedAt"]}
    if command["response"] == "declined":
        return {"event": "RecoveryOfferDeclined", "offerId": command["offerId"], "travelerId": command["travelerId"], "declinedAt": command["respondedAt"]}
    return {"event": "RecoveryOfferResponseRejected", "offerId": command["offerId"], "travelerId": command["travelerId"], "reason": "invalid_response"}


def project_marketing_conversion_report(responses: list[dict]) -> dict:
    return {"offerId": responses[0]["offerId"] if responses else "", "acceptedCount": sum(event.get("event") == "RecoveryOfferAccepted" for event in responses), "declinedCount": sum(event.get("event") == "RecoveryOfferDeclined" for event in responses)}
