package campus

import (
	"context"
	"fmt"
	"time"
)

// DeviceNode represents a device in the dependency tree.
type DeviceNode struct {
	DeviceID       int64        `json:"deviceId"`
	Name           string       `json:"name"`
	Host           string       `json:"host"`
	Status         string       `json:"status"`
	Category       string       `json:"category,omitempty"`
	LocationID     *int64       `json:"locationId,omitempty"`
	ParentDeviceID *int64       `json:"parentDeviceId,omitempty"`
	DependencyPort string       `json:"dependencyPort,omitempty"`
	Children       []*DeviceNode `json:"children,omitempty"`
}

// SuppressionResult indicates whether an alert should be suppressed for a device.
type SuppressionResult struct {
	ShouldSuppress  bool        `json:"shouldSuppress"`
	Reason          string      `json:"reason"`
	RootCauseDevice *DeviceNode `json:"rootCauseDevice,omitempty"`
	Message         string      `json:"message"`
}

// RootCauseOutage groups a down device with its affected dependants.
type RootCauseOutage struct {
	Device               DeviceNode     `json:"device"`
	AffectedCount        int            `json:"affectedCount"`
	AffectedByCategory   map[string]int `json:"affectedByCategory,omitempty"`
	SuppressedAlertCount int            `json:"suppressedAlertCount"`
	StartedAt            time.Time      `json:"startedAt"`
}

// TopologyService provides dependency tree analysis and alert suppression logic.
type TopologyService struct {
	db DB
}

// NewTopologyService creates a new TopologyService.
func NewTopologyService(db DB) *TopologyService {
	return &TopologyService{db: db}
}

// fetchAllDeviceNodes loads all devices as DeviceNode slices.
func (s *TopologyService) fetchAllDeviceNodes(ctx context.Context) ([]*DeviceNode, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, name, ip_address, status, device_category,
			location_id, parent_device_id, dependency_port
		FROM devices ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query devices: %w", err)
	}
	defer rows.Close()

	var nodes []*DeviceNode
	for rows.Next() {
		var n DeviceNode
		var cat, depPort *string
		if err := rows.Scan(
			&n.DeviceID, &n.Name, &n.Host, &n.Status,
			&cat, &n.LocationID, &n.ParentDeviceID, &depPort,
		); err != nil {
			return nil, err
		}
		if cat != nil {
			n.Category = *cat
		}
		if depPort != nil {
			n.DependencyPort = *depPort
		}
		n.Children = []*DeviceNode{}
		nodes = append(nodes, &n)
	}
	return nodes, rows.Err()
}

// BuildDependencyTree loads all devices and builds a parent-child tree.
// Root nodes are devices with no parent.
func (s *TopologyService) BuildDependencyTree(ctx context.Context) ([]*DeviceNode, error) {
	nodes, err := s.fetchAllDeviceNodes(ctx)
	if err != nil {
		return nil, err
	}

	byID := make(map[int64]*DeviceNode, len(nodes))
	for _, n := range nodes {
		byID[n.DeviceID] = n
	}

	var roots []*DeviceNode
	for _, n := range nodes {
		if n.ParentDeviceID == nil || byID[*n.ParentDeviceID] == nil {
			roots = append(roots, n)
			continue
		}
		parent := byID[*n.ParentDeviceID]
		parent.Children = append(parent.Children, n)
	}
	return roots, nil
}

// GetDeviceDependencies returns the ancestors (up to root) and all
// descendants of a given device.
func (s *TopologyService) GetDeviceDependencies(ctx context.Context, deviceID int64) (ancestors []*DeviceNode, descendants []*DeviceNode, err error) {
	nodes, err := s.fetchAllDeviceNodes(ctx)
	if err != nil {
		return nil, nil, err
	}

	byID := make(map[int64]*DeviceNode, len(nodes))
	for _, n := range nodes {
		byID[n.DeviceID] = n
	}

	// Walk up for ancestors.
	current := byID[deviceID]
	if current == nil {
		return nil, nil, fmt.Errorf("device %d not found", deviceID)
	}
	for current.ParentDeviceID != nil {
		parent := byID[*current.ParentDeviceID]
		if parent == nil {
			break
		}
		ancestors = append(ancestors, parent)
		current = parent
	}

	// Walk down for descendants (BFS).
	target := byID[deviceID]
	childrenOf := map[int64][]*DeviceNode{}
	for _, n := range nodes {
		if n.ParentDeviceID != nil {
			childrenOf[*n.ParentDeviceID] = append(childrenOf[*n.ParentDeviceID], n)
		}
	}
	queue := []*DeviceNode{target}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for _, child := range childrenOf[node.DeviceID] {
			descendants = append(descendants, child)
			queue = append(queue, child)
		}
	}

	return ancestors, descendants, nil
}

// CheckSuppression determines whether an alert for the given device should be
// suppressed because an ancestor device is down.
func (s *TopologyService) CheckSuppression(ctx context.Context, deviceID int64) (*SuppressionResult, error) {
	// Walk the parent chain by querying one device at a time.
	currentID := deviceID
	visited := map[int64]bool{deviceID: true}

	for {
		var parentID *int64
		var name, host, status string
		err := s.db.QueryRow(ctx,
			`SELECT name, ip_address, status, parent_device_id FROM devices WHERE id=$1`,
			currentID,
		).Scan(&name, &host, &status, &parentID)
		if err != nil {
			break // device not found or no parent
		}

		if parentID == nil {
			break // reached root, no suppression
		}

		// Check the parent's status.
		var pName, pHost, pStatus string
		var pParentID *int64
		err = s.db.QueryRow(ctx,
			`SELECT name, ip_address, status, parent_device_id FROM devices WHERE id=$1`,
			*parentID,
		).Scan(&pName, &pHost, &pStatus, &pParentID)
		if err != nil {
			break
		}

		if pStatus == "down" {
			return &SuppressionResult{
				ShouldSuppress: true,
				Reason:         "parent_down",
				RootCauseDevice: &DeviceNode{
					DeviceID: *parentID,
					Name:     pName,
					Host:     pHost,
					Status:   pStatus,
				},
				Message: fmt.Sprintf("Suppressed: parent device %s (%s) is down", pName, pHost),
			}, nil
		}

		// Prevent infinite loops.
		if visited[*parentID] {
			break
		}
		visited[*parentID] = true
		currentID = *parentID
	}

	return &SuppressionResult{
		ShouldSuppress: false,
		Reason:         "",
		Message:        "No suppression: all ancestors are up",
	}, nil
}

// GetRootCauseOutages finds all devices that are down AND have dependent
// children, counting the total affected devices per root cause.
func (s *TopologyService) GetRootCauseOutages(ctx context.Context) ([]RootCauseOutage, error) {
	nodes, err := s.fetchAllDeviceNodes(ctx)
	if err != nil {
		return nil, err
	}

	// Build parent-child index.
	childrenOf := map[int64][]*DeviceNode{}
	byID := map[int64]*DeviceNode{}
	for _, n := range nodes {
		byID[n.DeviceID] = n
		if n.ParentDeviceID != nil {
			childrenOf[*n.ParentDeviceID] = append(childrenOf[*n.ParentDeviceID], n)
		}
	}

	// A device is a root cause if it's down AND either has no parent,
	// or its parent is NOT down.
	var outages []RootCauseOutage
	for _, n := range nodes {
		if n.Status != "down" {
			continue
		}
		// Check if parent is also down (if so, this is not the root cause).
		if n.ParentDeviceID != nil {
			parent := byID[*n.ParentDeviceID]
			if parent != nil && parent.Status == "down" {
				continue // parent is also down, skip — it's the real root cause
			}
		}

		// Count affected descendant devices.
		affected := 0
		byCategory := map[string]int{}
		queue := []*DeviceNode{n}
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			for _, child := range childrenOf[curr.DeviceID] {
				affected++
				if child.Category != "" {
					byCategory[child.Category]++
				}
				queue = append(queue, child)
			}
		}

		if affected > 0 || len(childrenOf[n.DeviceID]) > 0 {
			outages = append(outages, RootCauseOutage{
				Device: DeviceNode{
					DeviceID:   n.DeviceID,
					Name:       n.Name,
					Host:       n.Host,
					Status:     n.Status,
					Category:   n.Category,
					LocationID: n.LocationID,
				},
				AffectedCount:        affected,
				AffectedByCategory:   byCategory,
				SuppressedAlertCount: affected,
				StartedAt:            time.Now(), // Would come from alert timestamp in production
			})
		}
	}
	return outages, nil
}
