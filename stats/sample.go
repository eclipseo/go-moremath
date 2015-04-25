// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stats

import (
	"math"
	"sort"
)

// Sample is a collection of possibly weighted data points.
type Sample struct {
	// Xs is the slice of sample values.
	Xs []float64

	// Weights[i] is the weight of sample Xs[i].  If Weights is
	// nil, all Xs have weight 1.  Weights must have the same
	// length of Xs and all values must be non-negative.
	Weights []float64

	// Sorted indicates that Xs is sorted in ascending order.
	Sorted bool
}

// Bounds returns the minimum and maximum values of xs.
func Bounds(xs []float64) (min float64, max float64) {
	if len(xs) == 0 {
		return math.NaN(), math.NaN()
	}
	min, max = xs[0], xs[0]
	for _, x := range xs {
		if x < min {
			min = x
		}
		if x > max {
			max = x
		}
	}
	return
}

// Bounds returns the minimum and maximum values of the Sample.
//
// If the Sample is weighted, this ignores samples with zero weight.
//
// This is constant time if s.Sorted and there are no zero-weighted
// values.
func (s Sample) Bounds() (min float64, max float64) {
	if len(s.Xs) == 0 || (!s.Sorted && s.Weights == nil) {
		return Bounds(s.Xs)
	}

	if s.Sorted {
		if s.Weights == nil {
			return s.Xs[0], s.Xs[len(s.Xs)-1]
		}
		min, max = math.NaN(), math.NaN()
		for i, w := range s.Weights {
			if w != 0 {
				min = s.Xs[i]
				break
			}
		}
		if math.IsNaN(min) {
			return
		}
		for i := range s.Weights {
			if s.Weights[len(s.Weights)-i-1] != 0 {
				max = s.Xs[len(s.Weights)-i-1]
				break
			}
		}
	} else {
		min, max = math.Inf(1), math.Inf(-1)
		for i, x := range s.Xs {
			w := s.Weights[i]
			if x < min && w != 0 {
				min = x
			}
			if x > max && w != 0 {
				max = x
			}
		}
		if math.IsInf(min, 0) {
			min, max = math.NaN(), math.NaN()
		}
	}
	return
}

// Sum returns the sum of xs.
func Sum(xs []float64) float64 {
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum
}

// Sum returns the (possibly weighted) sum of the Sample.
func (s Sample) Sum() float64 {
	if s.Weights == nil {
		return Sum(s.Xs)
	}
	sum := 0.0
	for i, x := range s.Xs {
		sum += x * s.Weights[i]
	}
	return sum
}

// Weight returns the total weight of the Sasmple.
func (s Sample) Weight() float64 {
	if s.Weights == nil {
		return float64(len(s.Xs))
	}
	return Sum(s.Weights)
}

// Mean returns the arithmetic mean of xs.
func Mean(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	m := 0.0
	for i, x := range xs {
		m += (x - m) / float64(i+1)
	}
	return m
}

// Mean returns the arithmetic mean of the Sample.
func (s Sample) Mean() float64 {
	if len(s.Xs) == 0 || s.Weights == nil {
		return Mean(s.Xs)
	}

	m, wsum := 0.0, 0.0
	for i, x := range s.Xs {
		// Use weighted incremental mean:
		//   m_i = (1 - w_i/wsum_i) * m_(i-1) + (w_i/wsum_i) * x_i
		//       = m_(i-1) + (x_i - m_(i-1)) * (w_i/wsum_i)
		w := s.Weights[i]
		wsum += w
		m += (x - m) * w / wsum
	}
	return m
}

// StdDev returns the sample standard deviation of xs.
func StdDev(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}

	// Based on Wikipedia's presentation of Welford 1962.  This is
	// more numerically stable than the standard two-pass formula.
	A, Q, k := 0.0, 0.0, 0
	for _, x := range xs {
		Anext := A + (x-A)/float64(k)
		Q += (x - A) * (x - Anext)
		A = Anext
		k++
	}
	return math.Sqrt(Q / float64(k-1))
}

// StdDev returns the sample standard deviation of the Sample.
func (s Sample) StdDev() float64 {
	if len(s.Xs) == 0 || s.Weights == nil {
		return StdDev(s.Xs)
	}
	// TODO(austin)
	panic("Weighted StdDev not implemented")
}

// Percentile returns the pctileth value from the Sample.
//
// pctile will be capped to the range [0, 1].  If len(xs) == 0 or all
// weights are 0, returns NaN.
//
// This is constant time if s.Sorted and s.Weights == nil.
func (s Sample) Percentile(pctile float64) float64 {
	if len(s.Xs) == 0 {
		return math.NaN()
	} else if pctile <= 0 {
		min, _ := s.Bounds()
		return min
	} else if pctile >= 1 {
		_, max := s.Bounds()
		return max
	}

	if !s.Sorted {
		// TODO(austin) Use select algorithm instead
		s = *s.Copy().Sort()
	}

	if s.Weights == nil {
		return s.Xs[int(pctile*float64(len(s.Xs)-1))]
	} else {
		target := s.Weight() * pctile

		// TODO(austin) If we had cumulative weights, we could
		// do this in log time.
		for i, weight := range s.Weights {
			target -= weight
			if target < 0 {
				return s.Xs[i]
			}
		}
		return s.Xs[len(s.Xs)-1]
	}
}

// IQR returns the interquartile range of the Sample.
//
// This is constant time if s.Sorted and s.Weights == nil.
func (s Sample) IQR() float64 {
	if !s.Sorted {
		s = *s.Copy().Sort()
	}
	return s.Percentile(0.75) - s.Percentile(0.25)
}

type sampleSorter struct {
	xs      []float64
	weights []float64
}

func (p *sampleSorter) Len() int {
	return len(p.xs)
}

func (p *sampleSorter) Less(i, j int) bool {
	return p.xs[i] < p.xs[j]
}

func (p *sampleSorter) Swap(i, j int) {
	p.xs[i], p.xs[j] = p.xs[j], p.xs[i]
	p.weights[i], p.weights[j] = p.weights[j], p.weights[i]
}

// Sort sorts the samples in place in s and returns s.
//
// A sorted sample improves the performance of some algorithms.
func (s *Sample) Sort() *Sample {
	if s.Sorted || sort.Float64sAreSorted(s.Xs) {
		// All set
	} else if s.Weights == nil {
		sort.Float64s(s.Xs)
	} else {
		sort.Sort(&sampleSorter{s.Xs, s.Weights})
	}
	s.Sorted = true
	return s
}

// Copy returns a copy of the Sample.
//
// The returned Sample shares no data with the original, so they can
// be modified (for example, sorted) independently.
func (s Sample) Copy() *Sample {
	xs := make([]float64, len(s.Xs))
	copy(xs, s.Xs)

	weights := []float64(nil)
	if s.Weights != nil {
		weights = make([]float64, len(s.Weights))
		copy(weights, s.Weights)
	}

	return &Sample{xs, weights, s.Sorted}
}
