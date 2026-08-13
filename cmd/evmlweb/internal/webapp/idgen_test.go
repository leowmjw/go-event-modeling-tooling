package webapp

import "testing"

func TestSlugify(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple", "Hotel Booking", "hotel-booking"},
		{"punctuation", "Hotel Booking!", "hotel-booking"},
		{"already slug", "hotel-booking", "hotel-booking"},
		{"leading/trailing space", "  Flight Arrival  ", "flight-arrival"},
		{"multiple separators", "Order__Fulfillment  Flow", "order-fulfillment-flow"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Slugify(tt.in); got != tt.want {
				t.Fatalf("Slugify(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDraftIDRoundTrip(t *testing.T) {
	tests := []struct {
		flow string
		date string
		seq  int
	}{
		{"hotel-booking", "2026-08-10", 1},
		{"hotel-booking", "2026-08-10", 12},
		{"flight-arrival-post-flight-settlement", "2026-12-31", 3},
	}
	for _, tt := range tests {
		id := NewDraftID(tt.flow, tt.date, tt.seq)
		gotFlow, gotDate, gotSeq, ok := ParseDraftID(id)
		if !ok {
			t.Fatalf("ParseDraftID(%q) returned ok=false", id)
		}
		if gotFlow != tt.flow || gotDate != tt.date || gotSeq != tt.seq {
			t.Fatalf("ParseDraftID(%q) = (%q, %q, %d), want (%q, %q, %d)",
				id, gotFlow, gotDate, gotSeq, tt.flow, tt.date, tt.seq)
		}
	}
}

func TestParseDraftIDRejectsGarbage(t *testing.T) {
	for _, bad := range []string{"", "hotel-booking", "hotel-booking-2026-08-10", "hotel-booking-v1", "not-a-date-v1"} {
		if _, _, _, ok := ParseDraftID(bad); ok {
			t.Fatalf("ParseDraftID(%q) unexpectedly succeeded", bad)
		}
	}
}
