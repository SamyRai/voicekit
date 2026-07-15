package evaluation

import "testing"

func TestHungarianAssignMaxWeightRecoversKnownPermutation(t *testing.T) {
	// Diagonal-dominant 3x3 matrix with an unambiguous optimal assignment:
	// row0->col0, row1->col1, row2->col2, total weight 45.
	weights := [][]float64{
		{10, 2, 0},
		{1, 15, 3},
		{0, 1, 20},
	}

	assignment := hungarianAssignMaxWeight(weights)

	want := []int{0, 1, 2}
	if len(assignment) != len(want) {
		t.Fatalf("expected %d assignments, got %d: %v", len(want), len(assignment), assignment)
	}
	for i, col := range want {
		if assignment[i] != col {
			t.Fatalf("expected row %d -> col %d, got assignment %v", i, col, assignment)
		}
	}

	total := 0.0
	for i, col := range assignment {
		total += weights[i][col]
	}
	if total != 45 {
		t.Fatalf("expected total weight 45, got %f", total)
	}
}

func TestHungarianAssignMaxWeightHandlesRectangularInput(t *testing.T) {
	// 3 rows (hyp speakers), 2 columns (ref speakers): one row must be left
	// unmatched (-1), and the matching that maximizes total weight assigns
	// row0->col0 (10) and row1->col1 (9), leaving row2 unmatched.
	weights := [][]float64{
		{10, 0},
		{0, 9},
		{1, 1},
	}

	assignment := hungarianAssignMaxWeight(weights)
	if len(assignment) != 3 {
		t.Fatalf("expected 3 row assignments, got %d: %v", len(assignment), assignment)
	}
	if assignment[0] != 0 || assignment[1] != 1 {
		t.Fatalf("expected row0->col0 and row1->col1, got %v", assignment)
	}
	if assignment[2] != -1 {
		t.Fatalf("expected row2 to be unmatched, got %v", assignment)
	}
}

func TestOptimalSpeakerMappingRecoversUnambiguousPairing(t *testing.T) {
	overlap := map[string]map[string]float64{
		"X": {"A": 9, "B": 0},
		"Y": {"A": 0, "B": 9},
	}

	mapping := optimalSpeakerMapping(overlap)

	if len(mapping) != 2 {
		t.Fatalf("expected 2 mapped speakers, got %d: %v", len(mapping), mapping)
	}
	if mapping["X"] != "A" || mapping["Y"] != "B" {
		t.Fatalf("expected X->A and Y->B, got %v", mapping)
	}
}

func TestOptimalSpeakerMappingOmitsZeroOverlapPairs(t *testing.T) {
	overlap := map[string]map[string]float64{
		"X": {"A": 0, "B": 0},
	}

	mapping := optimalSpeakerMapping(overlap)
	if len(mapping) != 0 {
		t.Fatalf("expected no mapped pairs for all-zero overlap, got %v", mapping)
	}
}

func TestOptimalSpeakerMappingHandlesEmptyInput(t *testing.T) {
	mapping := optimalSpeakerMapping(nil)
	if len(mapping) != 0 {
		t.Fatalf("expected empty mapping for nil overlap, got %v", mapping)
	}
}
