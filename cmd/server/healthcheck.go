package main

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

const healthcheckTimeout = 3 * time.Second

// healthcheck fails unless the server listening on addr reports itself
// ready, its database included.
func healthcheck(addr string) error {
	client := http.Client{Timeout: healthcheckTimeout}
	resp, err := client.Get("http://" + localAddr(addr) + "/api/health/ready")
	if err != nil {
		return err
	}
	if err := resp.Body.Close(); err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server unhealthy: %s", resp.Status)
	}
	return nil
}

// localAddr returns the address to reach a server listening on addr from the
// same machine.
func localAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}
