package heavy

import (
	"fmt"
	"sort"

	"github.com/jo-cube/toolbox/internal/prob"
)

type Config struct {
	Top      int
	Capacity int
	Exact    bool
	Input    prob.InputOptions
}

type Result struct {
	Rank          int    `json:"rank"`
	Item          string `json:"item"`
	CountEstimate uint64 `json:"count_estimate"`
}

func Run(paths []string, cfg Config) ([]Result, error) {
	if cfg.Top <= 0 {
		return nil, fmt.Errorf("top must be a positive integer")
	}
	if cfg.Capacity == 0 {
		cfg.Capacity = cfg.Top * 10
		if cfg.Capacity < 1000 {
			cfg.Capacity = 1000
		}
	}
	if cfg.Capacity < cfg.Top {
		return nil, fmt.Errorf("capacity must be at least top")
	}
	if cfg.Exact {
		return exact(paths, cfg)
	}
	return approximate(paths, cfg)
}

func exact(paths []string, cfg Config) ([]Result, error) {
	counts := map[string]uint64{}
	if err := prob.EachInput(paths, cfg.Input, func(item []byte) error {
		counts[string(item)]++
		return nil
	}); err != nil {
		return nil, err
	}
	return ranked(counts, cfg.Top), nil
}

func approximate(paths []string, cfg Config) ([]Result, error) {
	counts := map[string]uint64{}
	if err := prob.EachInput(paths, cfg.Input, func(item []byte) error {
		key := string(item)
		if _, ok := counts[key]; ok {
			counts[key]++
			return nil
		}
		if len(counts) < cfg.Capacity {
			counts[key] = 1
			return nil
		}

		minKey := ""
		var minCount uint64
		first := true
		for k, count := range counts {
			if first || count < minCount || count == minCount && k < minKey {
				minKey = k
				minCount = count
				first = false
			}
		}
		delete(counts, minKey)
		counts[key] = minCount + 1
		return nil
	}); err != nil {
		return nil, err
	}
	return ranked(counts, cfg.Top), nil
}

func ranked(counts map[string]uint64, top int) []Result {
	items := make([]Result, 0, len(counts))
	for item, count := range counts {
		items = append(items, Result{Item: item, CountEstimate: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CountEstimate == items[j].CountEstimate {
			return items[i].Item < items[j].Item
		}
		return items[i].CountEstimate > items[j].CountEstimate
	})
	if len(items) > top {
		items = items[:top]
	}
	for i := range items {
		items[i].Rank = i + 1
	}
	return items
}
