package tracking

import (
	"math"

	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

type Assignment struct {
	RowToCol []int
	Cost     float64
}

//gocyclo:ignore
func Hungarian(costMatrix [][]float64) *Assignment {
	n := len(costMatrix)
	if n == 0 {
		return &Assignment{RowToCol: []int{}, Cost: 0}
	}

	m := len(costMatrix[0])
	if m == 0 {
		return &Assignment{RowToCol: []int{}, Cost: 0}
	}

	size := n
	if m > n {
		size = m
	}

	cost := make([][]float64, size)
	for i := 0; i < size; i++ {
		cost[i] = make([]float64, size)
		for j := 0; j < size; j++ {
			if i < n && j < m {
				cost[i][j] = costMatrix[i][j]
			} else {
				cost[i][j] = 1e10
			}
		}
	}

	u := make([]float64, size+1)
	v := make([]float64, size+1)
	p := make([]int, size+1)
	way := make([]int, size+1)

	for i := 1; i <= size; i++ {
		p[0] = i
		j0 := 0
		minv := make([]float64, size+1)
		used := make([]bool, size+1)
		for j := 0; j <= size; j++ {
			minv[j] = math.Inf(1)
			used[j] = false
		}

		for {
			used[j0] = true
			i0 := p[j0]
			delta := math.Inf(1)
			j1 := 0

			for j := 1; j <= size; j++ {
				if !used[j] {
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
			}

			for j := 0; j <= size; j++ {
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

		for {
			j1 := way[j0]
			p[j0] = p[j1]
			j0 = j1
			if j0 == 0 {
				break
			}
		}
	}

	assignment := make([]int, n)
	for i := range assignment {
		assignment[i] = -1
	}
	for j := 1; j <= size; j++ {
		if p[j] != 0 && p[j] <= n && j <= m {
			assignment[p[j]-1] = j - 1
		}
	}

	costValue := 0.0
	for i := 0; i < n; i++ {
		if assignment[i] >= 0 && assignment[i] < m {
			costValue += costMatrix[i][assignment[i]]
		}
	}

	return &Assignment{
		RowToCol: assignment,
		Cost:     costValue,
	}
}

func ComputeIoUCost(detections []Detection, tracks []Track, matchThresh float64) [][]float64 {
	n := len(detections)
	m := len(tracks)

	if n == 0 || m == 0 {
		return [][]float64{}
	}

	cost := make([][]float64, n)
	for i := 0; i < n; i++ {
		cost[i] = make([]float64, m)
		for j := 0; j < m; j++ {
			bbox1 := detections[i].Bbox
			bbox2 := tracks[j].Bbox

			iou := computeIoU(bbox1, bbox2)

			if iou >= matchThresh {
				cost[i][j] = 1.0 - iou
			} else {
				cost[i][j] = 1.0
			}
		}
	}

	return cost
}

func computeIoU(bbox1, bbox2 [4]int) float64 {
	x1 := utils.Max(bbox1[0], bbox2[0])
	y1 := utils.Max(bbox1[1], bbox2[1])
	x2 := utils.Min(bbox1[2], bbox2[2])
	y2 := utils.Min(bbox1[3], bbox2[3])

	if x2 <= x1 || y2 <= y1 {
		return 0
	}

	intersection := (x2 - x1) * (y2 - y1)

	area1 := (bbox1[2] - bbox1[0]) * (bbox1[3] - bbox1[1])
	area2 := (bbox2[2] - bbox2[0]) * (bbox2[3] - bbox2[1])

	union := area1 + area2 - intersection

	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}
