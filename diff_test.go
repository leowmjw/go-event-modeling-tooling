package evml

import "testing"

const diffBase = `eventmodeling
tf 01 ui Payments.SendMoneyScreen
tf 02 cmd Payments.InitiatePayment ->> 01 { paymentId: "p-1", amount: 25.00 }
tf 03 evt Payments.PaymentInitiated ->> 02
tf 04 pcr Payments.PaymentAuthorizer ->> 03
tf 05 cmd Payments.ExecutePayment ->> 04
`

const diffNext = `eventmodeling
tf 01 ui Payments.SendMoneyScreen
tf 02 cmd Payments.InitiatePayment ->> 01 { paymentId: "p-1", amount: 25.00 }
tf 03 evt Payments.PaymentInitiated ->> 02
tf 04 pcr Payments.PaymentAuthorizer ->> 03 ->> 09
tf 05 cmd Payments.ExecutePayment ->> 04 { paymentId: "p-1" }
tf 09 rmo Risk.PaymentVelocity ->> 03
`

func mustParse(t *testing.T, src string) *Model {
	t.Helper()
	m, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	return m
}

func TestDiffClassifiesFrames(t *testing.T) {
	d := Diff(mustParse(t, diffBase), mustParse(t, diffNext))
	if d.Frames["01"] != DiffUnchanged {
		t.Fatalf("frame 01 = %s, want unchanged", d.Frames["01"])
	}
	if d.Frames["02"] != DiffUnchanged {
		t.Fatalf("frame 02 = %s, want unchanged", d.Frames["02"])
	}
	// 04 gains a source, 05 gains a payload: both changed.
	if d.Frames["04"] != DiffChanged || d.Frames["05"] != DiffChanged {
		t.Fatalf("frames 04/05 = %s/%s, want changed/changed", d.Frames["04"], d.Frames["05"])
	}
	if d.Frames["09"] != DiffAdded {
		t.Fatalf("frame 09 = %s, want added", d.Frames["09"])
	}
	if d.Added != 1 || d.Changed != 2 || d.Unchanged != 3 {
		t.Fatalf("counts = +%d ~%d =%d, want +1 ~2 =3", d.Added, d.Changed, d.Unchanged)
	}
}

func TestDiffRemovedFrames(t *testing.T) {
	d := Diff(mustParse(t, diffNext), mustParse(t, diffBase))
	if d.Frames["09"] != DiffRemoved {
		t.Fatalf("frame 09 = %s, want removed", d.Frames["09"])
	}
	if d.Removed != 1 || d.Changed != 2 || d.Unchanged != 3 || d.Added != 0 {
		t.Fatalf("counts = +%d -%d ~%d =%d, want +0 -1 ~2 =3", d.Added, d.Removed, d.Changed, d.Unchanged)
	}
}

func TestDiffIdenticalModels(t *testing.T) {
	m := mustParse(t, diffBase)
	d := Diff(m, mustParse(t, diffBase))
	if d.Added != 0 || d.Removed != 0 || d.Changed != 0 || d.Unchanged != len(m.Frames) {
		t.Fatalf("counts = +%d -%d ~%d =%d, want all unchanged", d.Added, d.Removed, d.Changed, d.Unchanged)
	}
}

func TestDiffSourceOrderInsensitive(t *testing.T) {
	base := `eventmodeling
tf 01 evt A
tf 02 evt B
tf 03 rmo Read ->> 01 ->> 02
`
	next := `eventmodeling
tf 01 evt A
tf 02 evt B
tf 03 rmo Read ->> 02 ->> 01
`
	d := Diff(mustParse(t, base), mustParse(t, next))
	if d.Frames["03"] != DiffUnchanged {
		t.Fatalf("frame 03 = %s, want unchanged (source order should not matter)", d.Frames["03"])
	}
}

func TestDiffNilModels(t *testing.T) {
	d := Diff(nil, nil)
	if len(d.Frames) != 0 {
		t.Fatalf("Frames = %v, want empty", d.Frames)
	}
}
