package git

import (
	"math/rand"
	"reflect"
	"testing"
)

// TestWaterFillLevel pins the FR3i water-fill level solver contract (PRD §9.1 FR3i;
// architecture/git_diff_semantics.md §6): the §6 sort-and-walk, the no-truncation first check, and the
// integer floor-division level. Expectations are HARDCODED (never derived from the function — that would be
// circular and couldn't catch a wrong formula). Pure table test; no git repo, no I/O.
func TestWaterFillLevel(t *testing.T) {
	tests := []struct {
		sizes     []int
		budget    int
		wantLevel int
		wantTrunc bool
		desc      string
	}{
		{
			sizes: []int{10, 20, 30}, budget: 30, wantLevel: 10, wantTrunc: true,
			desc: "§6 verified [10,20,30]@30 → (10, true)",
		},
		{
			sizes: []int{5, 100}, budget: 50, wantLevel: 45, wantTrunc: true,
			desc: "§6 verified [5,100]@50 → (45, true)",
		},
		{
			sizes: []int{10, 10}, budget: 50, wantLevel: 10, wantTrunc: false,
			desc: "§6 verified [10,10]@50 → (10, false) (total ≤ budget)",
		},
		{
			sizes: []int{1, 2, 3}, budget: 100, wantLevel: 3, wantTrunc: false,
			desc: "total < budget → (max=3, false)",
		},
		{
			sizes: []int{5}, budget: 5, wantLevel: 5, wantTrunc: false,
			desc: "total == budget → (max=5, false) (boundary: no truncation)",
		},
		{
			sizes: []int{7, 7, 7}, budget: 10, wantLevel: 3, wantTrunc: true,
			desc: "all-equal [7,7,7]@10 → (3, true) (floor(10/3); Σ=9 slack 1<3)",
		},
		{
			sizes: []int{100}, budget: 30, wantLevel: 30, wantTrunc: true,
			desc: "single file larger than budget [100]@30 → (30, true)",
		},
		{
			sizes: []int{10, 10, 100}, budget: 50, wantLevel: 30, wantTrunc: true,
			desc: "one file larger [10,10,100]@50 → (30, true)",
		},
		{
			sizes: []int{5, 10}, budget: 0, wantLevel: 0, wantTrunc: true,
			desc: "B=0 [5,10]@0 → (0, true)",
		},
		{
			sizes: []int{}, budget: 50, wantLevel: 0, wantTrunc: false,
			desc: "empty sizes @50 → (0, false)",
		},
		{
			sizes: []int{}, budget: 0, wantLevel: 0, wantTrunc: false,
			desc: "empty sizes @0 → (0, false)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			gotLevel, gotTrunc := waterFillLevel(tc.sizes, tc.budget)
			if gotLevel != tc.wantLevel || gotTrunc != tc.wantTrunc {
				t.Errorf("waterFillLevel(%v, %d) = (%d, %t), want (%d, %t)",
					tc.sizes, tc.budget, gotLevel, gotTrunc, tc.wantLevel, tc.wantTrunc)
			}
		})
	}
}

// TestWaterFillLevel_DoesNotMutateInput asserts the solver sorts a COPY — the caller's slice is left intact
// (Go slices are references; in-place sorting would surprise the caller, which reuses sizes for the
// min(size_i, level) map in allocByWaterFill).
func TestWaterFillLevel_DoesNotMutateInput(t *testing.T) {
	sizes := []int{3, 1, 2}
	before := append([]int(nil), sizes...)
	_, _ = waterFillLevel(sizes, 2) // budget < total ⇒ triggers the sort-and-walk
	if !reflect.DeepEqual(sizes, before) {
		t.Errorf("waterFillLevel mutated input: got %v, want %v (caller slice must be unchanged)", sizes, before)
	}
}

// TestAllocByWaterFill pins the FR3i water-fill allotment contract (PRD §9.1 FR3i; §6): out[i] = min(sizes[i],
// level) in INPUT ORDER (the output parallels the input — NOT sorted; the caller reassociates allotments to
// files BY INDEX). Expectations are HARDCODED. Pure table test; no git repo, no I/O.
func TestAllocByWaterFill(t *testing.T) {
	tests := []struct {
		sizes  []int
		budget int
		want   []int
		desc   string
	}{
		{
			sizes: []int{10, 20, 30}, budget: 30, want: []int{10, 10, 10},
			desc: "§6 verified [10,20,30]@30 → [10,10,10]",
		},
		{
			sizes: []int{5, 100}, budget: 50, want: []int{5, 45},
			desc: "§6 verified [5,100]@50 → [5,45]",
		},
		{
			sizes: []int{10, 10}, budget: 50, want: []int{10, 10},
			desc: "no truncation [10,10]@50 → [10,10] (whole)",
		},
		{
			sizes: []int{100, 5}, budget: 50, want: []int{45, 5},
			desc: "ORDER PRESERVED unsorted [100,5]@50 → [45,5] (NOT [5,45])",
		},
		{
			sizes: []int{30, 10, 20}, budget: 30, want: []int{10, 10, 10},
			desc: "ORDER PRESERVED unsorted [30,10,20]@30 → [10,10,10] (input order, all capped)",
		},
		{
			sizes: []int{7, 7, 7}, budget: 10, want: []int{3, 3, 3},
			desc: "all-equal [7,7,7]@10 → [3,3,3] (Σ=9 slack 1<3)",
		},
		{
			sizes: []int{100}, budget: 30, want: []int{30},
			desc: "single file larger [100]@30 → [30]",
		},
		{
			sizes: []int{5, 10}, budget: 0, want: []int{0, 0},
			desc: "B=0 [5,10]@0 → [0,0]",
		},
		{
			sizes: []int{}, budget: 50, want: []int{},
			desc: "empty sizes @50 → [] (len 0)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got := allocByWaterFill(tc.sizes, tc.budget)
			if len(got) != len(tc.sizes) {
				t.Errorf("allocByWaterFill(%v, %d) length = %d, want %d (length parity with input)",
					tc.sizes, tc.budget, len(got), len(tc.sizes))
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("allocByWaterFill(%v, %d) = %v, want %v", tc.sizes, tc.budget, got, tc.want)
			}
		})
	}
}

// TestWaterFill_PropertyInvariants is the FR3i randomized property loop (PRD §9.1 FR3i; §6). It uses a FIXED
// seed + NON-NEGATIVE sizes/budgets (production values are token counts) over 1000 cases to assert the five
// invariants: (1) Σ allotments ≤ budget; (2) allot_i ≤ size_i ∀i; (3) allot_i ≥ 0 ∀i; (4) no-trunc ⇒
// allot_i == size_i ∀i; (5) truncated ⇒ 0 ≤ (budget − Σ allots) < N. Deterministic (seed 1) ⇒ reproducible.
func TestWaterFill_PropertyInvariants(t *testing.T) {
	r := rand.New(rand.NewSource(1)) // fixed seed — deterministic
	const iters = 1000
	for iter := 0; iter < iters; iter++ {
		n := r.Intn(10) // 0..9 files
		sizes := make([]int, n)
		for i := range sizes {
			sizes[i] = r.Intn(200) // 0..199 tokens each
		}
		budget := r.Intn(300) // 0..299 tokens

		level, trunc := waterFillLevel(sizes, budget)
		allots := allocByWaterFill(sizes, budget)

		// Length parity.
		if len(allots) != len(sizes) {
			t.Fatalf("iter %d: len(allots)=%d != len(sizes)=%d", iter, len(allots), len(sizes))
		}

		sum := 0
		for i, s := range sizes {
			// (2) allotment_i ≤ size_i; (3) allotment_i ≥ 0.
			if allots[i] < 0 || allots[i] > s {
				t.Fatalf("iter %d: allot[%d]=%d out of [0, size=%d]", iter, i, allots[i], s)
			}
			// (4) no-trunc ⇒ allotment_i == size_i.
			if !trunc && allots[i] != s {
				t.Fatalf("iter %d: no-trunc but allots[%d]=%d != size=%d", iter, i, allots[i], s)
			}
			// Level consistency: every allot is min(size, level).
			want := s
			if level < s {
				want = level
			}
			if allots[i] != want {
				t.Fatalf("iter %d: allots[%d]=%d != min(size=%d, level=%d)=%d", iter, i, allots[i], s, level, want)
			}
			sum += allots[i]
		}

		// (1) Σ allotments ≤ budget.
		if sum > budget {
			t.Fatalf("iter %d: Σ allots=%d > budget=%d", iter, sum, budget)
		}

		// (5) if truncated, 0 ≤ slack < N.
		if trunc {
			slack := budget - sum
			if slack < 0 || slack >= len(sizes) {
				t.Fatalf("iter %d: truncated but slack=%d out of [0, N=%d)", iter, slack, len(sizes))
			}
		}
	}
}
