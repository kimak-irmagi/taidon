package prepare

import "testing"

func TestStateHasherDeterministic(t *testing.T) {
	hasher1 := newStateHasher()
	hasher1.write("key", "value")
	sum1 := hasher1.sum()

	hasher2 := newStateHasher()
	hasher2.write("key", "value")
	sum2 := hasher2.sum()

	if sum1 != sum2 {
		t.Fatalf("expected deterministic hash, got %q vs %q", sum1, sum2)
	}
}

func TestStateHasherDifferentInput(t *testing.T) {
	hasher1 := newStateHasher()
	hasher1.write("key", "value")
	sum1 := hasher1.sum()

	hasher2 := newStateHasher()
	hasher2.write("key", "other")
	sum2 := hasher2.sum()

	if sum1 == sum2 {
		t.Fatalf("expected different hashes")
	}
}
