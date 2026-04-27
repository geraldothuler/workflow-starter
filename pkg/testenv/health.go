package testenv

import (
	"fmt"
	"net"
	"time"
)

// WaitForServices polls each service port until healthy or timeout.
func WaitForServices(services []ServiceConfig, cfg *HealthConfig) []ServiceHealth {
	results := make([]ServiceHealth, len(services))
	for i, svc := range services {
		results[i] = waitForService(svc, cfg)
	}
	return results
}

func waitForService(svc ServiceConfig, cfg *HealthConfig) ServiceHealth {
	start := time.Now()
	interval := time.Duration(cfg.IntervalSeconds) * time.Second
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	deadline := start.Add(timeout)

	for time.Now().Before(deadline) {
		if tcpReachable(svc.Port) {
			return ServiceHealth{
				Name:    svc.Name,
				Port:    svc.Port,
				Status:  "healthy",
				Elapsed: time.Since(start).Round(time.Millisecond).String(),
			}
		}
		time.Sleep(interval)
	}
	return ServiceHealth{
		Name:    svc.Name,
		Port:    svc.Port,
		Status:  "timeout",
		Elapsed: time.Since(start).Round(time.Millisecond).String(),
	}
}

func tcpReachable(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
