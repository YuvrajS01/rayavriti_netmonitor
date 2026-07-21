package scheduler

import "sync"

type DependencyTree struct {
	children map[int64][]int64
	parents  map[int64]int64
	mu       sync.RWMutex
}

func NewDependencyTree() *DependencyTree {
	return &DependencyTree{
		children: make(map[int64][]int64),
		parents:  make(map[int64]int64),
	}
}

func (dt *DependencyTree) SetParent(deviceID, parentID int64) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	if oldParent, exists := dt.parents[deviceID]; exists {
		oldChildren := dt.children[oldParent]
		for i, id := range oldChildren {
			if id == deviceID {
				dt.children[oldParent] = append(oldChildren[:i], oldChildren[i+1:]...)
				break
			}
		}
		if len(dt.children[oldParent]) == 0 {
			delete(dt.children, oldParent)
		}
	}

	if parentID != 0 {
		dt.parents[deviceID] = parentID
		dt.children[parentID] = append(dt.children[parentID], deviceID)
	} else {
		delete(dt.parents, deviceID)
	}
}

func (dt *DependencyTree) RemoveDevice(deviceID int64) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	if parentID, exists := dt.parents[deviceID]; exists {
		children := dt.children[parentID]
		for i, id := range children {
			if id == deviceID {
				dt.children[parentID] = append(children[:i], children[i+1:]...)
				break
			}
		}
		if len(dt.children[parentID]) == 0 {
			delete(dt.children, parentID)
		}
	}
	delete(dt.parents, deviceID)
	delete(dt.children, deviceID)
}

func (dt *DependencyTree) GetParent(deviceID int64) (int64, bool) {
	dt.mu.RLock()
	defer dt.mu.RUnlock()
	parentID, exists := dt.parents[deviceID]
	return parentID, exists
}

func (dt *DependencyTree) GetChildren(parentID int64) []int64 {
	dt.mu.RLock()
	defer dt.mu.RUnlock()
	children := dt.children[parentID]
	result := make([]int64, len(children))
	copy(result, children)
	return result
}

func (dt *DependencyTree) GetDescendants(deviceID int64) []int64 {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	var result []int64
	dt.collectDescendants(deviceID, &result)
	return result
}

func (dt *DependencyTree) collectDescendants(deviceID int64, result *[]int64) {
	for _, childID := range dt.children[deviceID] {
		*result = append(*result, childID)
		dt.collectDescendants(childID, result)
	}
}

func (dt *DependencyTree) IsDependency(deviceID int64) bool {
	dt.mu.RLock()
	defer dt.mu.RUnlock()
	_, exists := dt.parents[deviceID]
	return exists
}

func (dt *DependencyTree) IsParent(deviceID int64) bool {
	dt.mu.RLock()
	defer dt.mu.RUnlock()
	_, exists := dt.children[deviceID]
	return exists && len(dt.children[deviceID]) > 0
}

func (dt *DependencyTree) GetAncestors(deviceID int64) []int64 {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	var ancestors []int64
	current := deviceID
	for {
		parentID, exists := dt.parents[current]
		if !exists {
			break
		}
		ancestors = append(ancestors, parentID)
		current = parentID
	}
	return ancestors
}

func (dt *DependencyTree) Count() int {
	dt.mu.RLock()
	defer dt.mu.RUnlock()
	return len(dt.parents)
}