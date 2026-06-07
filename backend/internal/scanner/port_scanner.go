package scanner

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// PortResult holds the result for a single port scan.
type PortResult struct {
	Port    int
	Open    bool
	Banner  string
}

// ScanOptions controls scanner behaviour.
type ScanOptions struct {
	Timeout     time.Duration
	Concurrency int
}

var DefaultOptions = ScanOptions{
	Timeout:     2 * time.Second,
	Concurrency: 100,
}

// ScanPorts performs a concurrent TCP connect scan on the given host/ports.
// Results are returned in the same order as the input slice.
func ScanPorts(ctx context.Context, host string, ports []int, opts ScanOptions) []PortResult {
	if opts.Concurrency <= 0 {
		opts.Concurrency = DefaultOptions.Concurrency
	}
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultOptions.Timeout
	}

	results := make([]PortResult, len(ports))
	sem := make(chan struct{}, opts.Concurrency)
	var wg sync.WaitGroup

	for i, port := range ports {
		wg.Add(1)
		go func(idx, p int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				results[idx] = PortResult{Port: p}
				return
			default:
			}

			addr := fmt.Sprintf("%s:%d", host, p)
			conn, err := net.DialTimeout("tcp", addr, opts.Timeout)
			if err != nil {
				results[idx] = PortResult{Port: p, Open: false}
				return
			}
			_ = conn.Close()
			results[idx] = PortResult{Port: p, Open: true}
		}(i, port)
	}
	wg.Wait()
	return results
}

// CommonPorts is a typical set of ports to scan during device discovery.
var CommonPorts = []int{
	21, 22, 23, 25, 53, 80, 110, 143, 161, 443,
	465, 587, 993, 995, 1433, 3306, 3389, 5432, 6379, 8080,
	8443, 27017,
}
