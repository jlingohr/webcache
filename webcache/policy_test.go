package webcache

import (
	"testing"
)

func Test_LRU_Promote_Single(t *testing.T) {
	policy := NewLRUPolicy()
	entry := NewEntry("testkey", []byte("testvalue"), 0)
	policy.Promote(entry)
	head := policy.entries.Front()
	if entry.Key != head.Value.(*Entry).Key {
		t.Errorf("Expected testket, got %s", head.Value.(*Entry).Key)
	}
}

func Test_LRU_Promot_Two(t *testing.T) {
	policy := NewLRUPolicy()
	keyA := "keyA"
	keyB := "keyB"
	entryA := NewEntry(keyA, []byte(keyA), 0)
	entryB := NewEntry(keyB, []byte(keyB), 0)
	policy.Promote(entryA)
	policy.Promote(entryB)
	head := policy.entries.Front()
	headKey := head.Value.(*Entry).Key
	if headKey != entryB.Key {
		t.Errorf("Expected %s, got %s", entryB.Key, headKey)
	}
}

func Test_LRU_Evict_Single(t *testing.T) {
	policy := NewLRUPolicy()
	keyA := "keyA"
	keyB := "keyB"
	entryA := NewEntry(keyA, []byte(keyA), 0)
	entryB := NewEntry(keyB, []byte(keyB), 0)
	policy.Promote(entryA)
	policy.Promote(entryB)
	evicted := policy.Evict()
	if evicted.Key != keyA {
		t.Errorf("Expected %s, got %s", entryA.Key, evicted.Key)
	}

	head := policy.entries.Front()
	headKey := head.Value.(*Entry).Key
	if keyB != headKey {
		t.Errorf("Expected %s, got %s", keyB, headKey)
	}

}

func Test_LRU_Promote_Twice(t *testing.T) {
	policy := NewLRUPolicy()
	keyA := "keyA"
	keyB := "keyB"
	entryA := NewEntry(keyA, []byte(keyA), 0)
	entryB := NewEntry(keyB, []byte(keyB), 0)
	policy.Promote(entryA)
	policy.Promote(entryB)
	policy.Promote(entryA)
	evicted := policy.Evict()
	if evicted.Key != keyB {
		t.Errorf("Expected %s, got %s", entryB.Key, evicted.Key)
	}

	head := policy.entries.Front()
	headKey := head.Value.(*Entry).Key
	if keyA != headKey {
		t.Errorf("Expected %s, got %s", keyA, headKey)
	}
}

////////////////
// LFU Tests

func Test_LFU_Promote_Single(t *testing.T) {
	policy := NewLFUPolicy()
	entry := NewEntry("testkey", []byte("testvalue"), 0)
	policy.Promote(entry)
	head := policy.entries.Pop()
	headKey := head.(*Entry).Key
	if entry.Key != headKey {
		t.Errorf("Expected testket, got %s", headKey)
	}
}

func Test_LFU_Promot_Two(t *testing.T) {
	policy := NewLFUPolicy()
	keyA := "keyA"
	keyB := "keyB"
	entryA := NewEntry(keyA, []byte(keyA), 0)
	entryB := NewEntry(keyB, []byte(keyB), 0)
	policy.Promote(entryA)
	policy.Promote(entryB)
	length := policy.entries.Len()
	if length != 2 {
		t.Errorf("Expected 2 items, only %d inserted", length)
	}
}

func Test_LFU_Evict_Single(t *testing.T) {
	policy := NewLFUPolicy()
	keyA := "keyA"
	keyB := "keyB"
	entryA := NewEntry(keyA, []byte(keyA), 0)
	entryB := NewEntry(keyB, []byte(keyB), 0)
	policy.Promote(entryA)
	policy.Promote(entryB)
	policy.Promote(entryB)
	evicted := policy.Evict()
	if evicted.Key != keyA {
		t.Errorf("Expected %s, got %s", entryA.Key, evicted.Key)
	}

	length := policy.entries.Len()
	if length != 1 {
		t.Errorf("Expected 2 items, only %d inserted", length)
	}

}
