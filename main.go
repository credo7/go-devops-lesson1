package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Metrics struct {
	LoadAverage                    int
	RamTotalBytes                  int
	RamUsageBytes                  int
	DiskTotalBytes                 int
	DiskUsageBytes                 int
	NetworkBandwidthBytesPerSecond int
	NetworkLoadBytesPerSecond      int
}

func main() {
	url := "http://srv.msk01.gigacorp.local/_stats"

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	failuresCount := 0

	for {
		select {
		case <-ticker.C:
			_ = checkMetrics(url, &failuresCount)
		case <-quit:
			log.Println("Shutting down...")
			return
		}
	}
}

func checkMetrics(url string, failuresCount *int) error {
	m, err := getMetrics(url)
	if err != nil {
		*failuresCount++
		if *failuresCount >= 3 {
			log.Println("Unable to fetch server statistic")
		}
		return err
	}

	*failuresCount = 0

	if m.LoadAverage > 30 {
		log.Printf("Load Average is too high: %v", m.LoadAverage)
	}

	ramUsagePercentage := calculatePercentage(m.RamUsageBytes, m.RamTotalBytes)
	if ramUsagePercentage > 80 {
		log.Printf("Memory usage too high: %.2f%%", ramUsagePercentage)
	}

	diskUsagePercentage := calculatePercentage(m.DiskUsageBytes, m.DiskTotalBytes)
	if diskUsagePercentage > 90 {
		leftDiskMemoryMB := (m.DiskTotalBytes - m.DiskUsageBytes) / (1024 * 1024)
		log.Printf("Free disk space is too low: %v MB left", leftDiskMemoryMB)
	}

	networkBandwidthUsagePercentage := calculatePercentage(m.NetworkLoadBytesPerSecond, m.NetworkBandwidthBytesPerSecond)
	if networkBandwidthUsagePercentage > 90 {
		leftNetworkBandwidthMb := (m.NetworkBandwidthBytesPerSecond - m.NetworkLoadBytesPerSecond) / (1024 * 1024) * 8
		log.Printf("Network bandwidth usage high: %.2f Mbit/s available", leftNetworkBandwidthMb)
	}

	return nil
}

func getMetrics(url string) (Metrics, error) {
	m := Metrics{}

	resp, err := http.Get(url)
	if err != nil {
		return m, fmt.Errorf("error fetching metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "text/plain" {
		return m, errors.New("Content-Type mismatch")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return m, fmt.Errorf("error reading response body: %w", err)
	}

	values := strings.Fields(string(body))
	if len(values) != 7 {
		return m, fmt.Errorf("unexpected number of metrics: expected 7, got %d", len(values))
	}

	err = parseMetrics(values, &m)
	if err != nil {
		return m, fmt.Errorf("error parsing metrics: %w", err)
	}

	return m, nil
}

func parseMetrics(values []string, m *Metrics) error {
	var err error
	if m.LoadAverage, err = strconv.Atoi(values[0]); err != nil {
		return fmt.Errorf("LoadAverage parsing error: %w", err)
	}
	if m.RamTotalBytes, err = strconv.Atoi(values[1]); err != nil {
		return fmt.Errorf("RamTotalBytes parsing error: %w", err)
	}
	if m.RamUsageBytes, err = strconv.Atoi(values[2]); err != nil {
		return fmt.Errorf("RamUsageBytes parsing error: %w", err)
	}
	if m.DiskTotalBytes, err = strconv.Atoi(values[3]); err != nil {
		return fmt.Errorf("DiskTotalBytes parsing error: %w", err)
	}
	if m.DiskUsageBytes, err = strconv.Atoi(values[4]); err != nil {
		return fmt.Errorf("DiskUsageBytes parsing error: %w", err)
	}
	if m.NetworkBandwidthBytesPerSecond, err = strconv.Atoi(values[5]); err != nil {
		return fmt.Errorf("NetworkBandwidthBytesPerSecond parsing error: %w", err)
	}
	if m.NetworkLoadBytesPerSecond, err = strconv.Atoi(values[6]); err != nil {
		return fmt.Errorf("NetworkLoadBytesPerSecond parsing error: %w", err)
	}
	return nil
}

func calculatePercentage(used, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}
