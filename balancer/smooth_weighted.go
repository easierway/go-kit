package balancer

type SmoothWeighted struct {
	len    int //service nums
	items  []*weightedItem
	indexs map[string]int
}

type weightedItem struct {
	val             string //service ip
	weight          int    //init weight
	currentWeight   int    //the weight of current round
	effectiveWeight int
}

//Add weighted server
func (sw *SmoothWeighted) Add(server string, weight int) {
	idx, ok := sw.indexs[server]
	if !ok {
		wItem := &weightedItem{val: server, weight: weight, effectiveWeight: weight}
		sw.items = append(sw.items, wItem)
		sw.len++
		return
	}
	wItem := sw.items[idx]
	if wItem != nil && wItem.weight != weight {
		wItem.weight = weight
		wItem.currentWeight = weight
		sw.items[idx] = wItem
	}
	return
}

func (sw *SmoothWeighted) GetLen() int {
	return sw.len
}

//clean all weighted
func (sw *SmoothWeighted) Clean() {
	sw.items = sw.items[:0]
	sw.len = 0
	sw.indexs = nil
}

//delete item
func (sw *SmoothWeighted) Delete(server string) {
	idx, ok := sw.indexs[server]
	if !ok {
		return
	}
	sw.items[idx] = sw.items[sw.len-1]
	sw.items[sw.len-1] = nil
	sw.items = sw.items[:sw.len-1]
	sw.len--
}

//reset current weight of all items
func (sw *SmoothWeighted) Reset() {
	for _, item := range sw.items {
		item.effectiveWeight = item.weight
		item.currentWeight = 0
	}
}

//return all items weight
func (sw *SmoothWeighted) All() map[string]int {
	result := make(map[string]int)
	for _, item := range sw.items {
		result[item.val] = item.weight
	}
	return result
}

func (sw *SmoothWeighted) Next() string {
	if sw.len == 0 {
		return ""
	}
	if sw.len == 1 {
		return sw.items[0].val
	}
	best := nextSmoothWeighted(sw.items)
	if best == nil {
		return ""
	}
	return best.val
}

func nextSmoothWeighted(items []*weightedItem) (best *weightedItem) {
	total := 0
	for i := 0; i < len(items); i++ {
		curItem := items[i]
		if curItem == nil {
			continue
		}
		total += curItem.effectiveWeight
		curItem.currentWeight += curItem.effectiveWeight
		if best == nil || curItem.currentWeight > best.currentWeight {
			best = curItem
		}
	}
	if best == nil {
		return nil
	}
	best.currentWeight -= total
	return best
}
