package screen

import "testing"

func TestFIFOEvictsOnlyKnownOldest(t *testing.T) {
	q := Queue{Entries: []QueueEntry{{CRC: 11, Slot: 0}, {CRC: 22, Slot: 1}}}
	if _, err := q.Plan(33, 8, 255); err == nil {
		t.Fatal("partial queue must refuse eviction")
	}
	for slot := 2; slot < 8; slot++ {
		q.Entries = append(q.Entries, QueueEntry{CRC: uint16(100 + slot), Slot: uint8(slot)})
	}
	victim, err := q.Plan(33, 8, 255)
	if err != nil || victim == nil || victim.CRC != 11 {
		t.Fatalf("%v %v", victim, err)
	}
	victim, err = q.Plan(22, 8, 255)
	if err != nil || victim != nil {
		t.Fatalf("duplicate should not evict: %v %v", victim, err)
	}
	if _, err := (&Queue{}).Plan(33, 8, 255); err == nil {
		t.Fatal("must not evict unknown images")
	}
	if _, err := q.Plan(33, 8, 254); err == nil {
		t.Fatal("missing oldest must fail closed")
	}
}
func TestQueueRecordDeduplicatesAndDropsEvicted(t *testing.T) {
	q := Queue{Entries: []QueueEntry{{CRC: 11, Slot: 0}, {CRC: 22, Slot: 1}}}
	q.Record(33, 0, 11)
	if len(q.Entries) != 2 || q.Entries[0].CRC != 22 || q.Entries[1].CRC != 33 {
		t.Fatal(q)
	}
	q.Record(22, 1, 0)
	if len(q.Entries) != 2 || q.Entries[0].CRC != 22 {
		t.Fatal("duplicate must keep original age")
	}
}
