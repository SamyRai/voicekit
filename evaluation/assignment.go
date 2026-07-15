package evaluation

import (
	"math"
	"sort"
)

// optimalSpeakerMapping computes the 1:1 hypothesis-to-reference speaker
// mapping that maximizes total overlap time, using the Hungarian
// (Kuhn-Munkres) algorithm. overlap is keyed by hypothesis speaker ID, then
// reference speaker ID, holding the total time both speakers were active
// simultaneously. The returned map only contains pairs with positive
// overlap; hypothesis or reference speakers with no beneficial pairing are
// omitted.
func optimalSpeakerMapping(overlap map[string]map[string]float64) map[string]string {
	hypSpeakers := make([]string, 0, len(overlap))
	for hyp := range overlap {
		hypSpeakers = append(hypSpeakers, hyp)
	}
	sort.Strings(hypSpeakers)

	refSet := make(map[string]struct{})
	for _, refs := range overlap {
		for ref := range refs {
			refSet[ref] = struct{}{}
		}
	}
	refSpeakers := make([]string, 0, len(refSet))
	for ref := range refSet {
		refSpeakers = append(refSpeakers, ref)
	}
	sort.Strings(refSpeakers)

	mapping := make(map[string]string, len(hypSpeakers))
	if len(hypSpeakers) == 0 || len(refSpeakers) == 0 {
		return mapping
	}

	weights := make([][]float64, len(hypSpeakers))
	for i, hyp := range hypSpeakers {
		weights[i] = make([]float64, len(refSpeakers))
		for j, ref := range refSpeakers {
			weights[i][j] = overlap[hyp][ref]
		}
	}

	assignment := hungarianAssignMaxWeight(weights)
	for i, col := range assignment {
		if col < 0 || weights[i][col] <= 0 {
			continue
		}
		mapping[hypSpeakers[i]] = refSpeakers[col]
	}
	return mapping
}

// hungarianAssignMaxWeight computes an optimal one-to-one assignment between
// rows and columns of weights that maximizes total assigned weight. It
// returns, for each row, the assigned column index, or -1 if the row has no
// matched column (only possible when there are fewer columns than rows).
// weights need not be square or rectangular-uniform; rows may outnumber
// columns or vice versa.
func hungarianAssignMaxWeight(weights [][]float64) []int {
	rows := len(weights)
	if rows == 0 {
		return nil
	}
	cols := 0
	for _, row := range weights {
		if len(row) > cols {
			cols = len(row)
		}
	}
	if cols == 0 {
		assignment := make([]int, rows)
		for i := range assignment {
			assignment[i] = -1
		}
		return assignment
	}

	n := rows
	if cols > n {
		n = cols
	}

	padded := make([][]float64, n)
	maxWeight := 0.0
	for i := 0; i < n; i++ {
		padded[i] = make([]float64, n)
		if i < rows {
			for j := 0; j < len(weights[i]) && j < n; j++ {
				padded[i][j] = weights[i][j]
				if padded[i][j] > maxWeight {
					maxWeight = padded[i][j]
				}
			}
		}
	}

	// Convert to a minimization problem: matching a real cell of maximum
	// weight costs 0, so the minimum-cost perfect matching on the padded
	// square matrix maximizes total weight over the original rectangle.
	cost := make([][]float64, n)
	for i := 0; i < n; i++ {
		cost[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			cost[i][j] = maxWeight - padded[i][j]
		}
	}

	matchColByRow := solveHungarian(cost)

	assignment := make([]int, rows)
	for i := 0; i < rows; i++ {
		col := matchColByRow[i]
		if col >= 0 && col < cols {
			assignment[i] = col
		} else {
			assignment[i] = -1
		}
	}
	return assignment
}

// solveHungarian returns the minimum-cost perfect matching on a square cost
// matrix using the O(n^3) Kuhn-Munkres algorithm with shortest-path
// potentials.
func solveHungarian(cost [][]float64) []int {
	n := len(cost)
	if n == 0 {
		return nil
	}

	const inf = math.MaxFloat64

	u := make([]float64, n+1)
	v := make([]float64, n+1)
	p := make([]int, n+1) // p[j] = row matched to column j (1-indexed row), 0 = none
	way := make([]int, n+1)

	for i := 1; i <= n; i++ {
		p[0] = i
		j0 := 0
		minv := make([]float64, n+1)
		used := make([]bool, n+1)
		for j := range minv {
			minv[j] = inf
		}

		for {
			used[j0] = true
			i0 := p[j0]
			delta := inf
			j1 := -1
			for j := 1; j <= n; j++ {
				if used[j] {
					continue
				}
				cur := cost[i0-1][j-1] - u[i0] - v[j]
				if cur < minv[j] {
					minv[j] = cur
					way[j] = j0
				}
				if minv[j] < delta {
					delta = minv[j]
					j1 = j
				}
			}
			for j := 0; j <= n; j++ {
				if used[j] {
					u[p[j]] += delta
					v[j] -= delta
				} else {
					minv[j] -= delta
				}
			}
			j0 = j1
			if p[j0] == 0 {
				break
			}
		}

		for j0 != 0 {
			j1 := way[j0]
			p[j0] = p[j1]
			j0 = j1
		}
	}

	matchColByRow := make([]int, n)
	for i := range matchColByRow {
		matchColByRow[i] = -1
	}
	for j := 1; j <= n; j++ {
		if p[j] != 0 {
			matchColByRow[p[j]-1] = j - 1
		}
	}
	return matchColByRow
}
