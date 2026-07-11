package heavy

import (
	"container/heap"
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

type trackedItem struct {
	item  string
	count uint64
	index int
}

type minItems []*trackedItem

func (h minItems) Len() int { return len(h) }
func (h minItems) Less(i, j int) bool {
	if h[i].count == h[j].count {
		return h[i].item < h[j].item
	}
	return h[i].count < h[j].count
}
func (h minItems) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *minItems) Push(value any) {
	item := value.(*trackedItem)
	item.index = len(*h)
	*h = append(*h, item)
}
func (h *minItems) Pop() any {
	old := *h
	last := len(old) - 1
	item := old[last]
	old[last] = nil
	item.index = -1
	*h = old[:last]
	return item
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
	tracked := map[string]*trackedItem{}
	var items minItems
	if err := prob.EachInput(paths, cfg.Input, func(item []byte) error {
		key := string(item)
		if existing, ok := tracked[key]; ok {
			existing.count++
			heap.Fix(&items, existing.index)
			return nil
		}
		if len(tracked) < cfg.Capacity {
			entry := &trackedItem{item: key, count: 1}
			tracked[key] = entry
			heap.Push(&items, entry)
			return nil
		}

		replaced := heap.Pop(&items).(*trackedItem)
		delete(tracked, replaced.item)
		replaced.item = key
		replaced.count++
		tracked[key] = replaced
		heap.Push(&items, replaced)
		return nil
	}); err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(tracked))
	for item, entry := range tracked {
		results = append(results, Result{Item: item, CountEstimate: entry.count})
	}
	return rank(results, cfg.Top), nil
}

func ranked(counts map[string]uint64, top int) []Result {
	items := make([]Result, 0, len(counts))
	for item, count := range counts {
		items = append(items, Result{Item: item, CountEstimate: count})
	}
	return rank(items, top)
}

func rank(items []Result, top int) []Result {
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
