package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
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
	RAMTotalBytes                  int
	RAMUsageBytes                  int
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
			return
		}
	}
}

func checkMetrics(url string, failuresCount *int) error {
	m, err := getMetrics(url)
	if err != nil {
		fmt.Println(err.Error())
		*failuresCount++
		if *failuresCount >= 3 {
			fmt.Printf("Unable to fetch server statistic\n")
		}
		return err
	}

	*failuresCount = 0

	if m.LoadAverage > 30 {
		fmt.Printf("Load Average is too high: %v\n", m.LoadAverage)
	}

	ramUsagePercentage := calculatePercentage(m.RAMUsageBytes, m.RAMTotalBytes)
	if ramUsagePercentage > 80 {
		fmt.Printf("Memory usage too high: %.0f%%\n", ramUsagePercentage)
	}

	diskUsagePercentage := calculatePercentage(m.DiskUsageBytes, m.DiskTotalBytes)
	if diskUsagePercentage > 90 {
		leftDiskMemoryMB := (m.DiskTotalBytes - m.DiskUsageBytes) / (1024 * 1024)
		fmt.Printf("Free disk space is too low: %v Mb left\n", leftDiskMemoryMB)
	}

	networkBandwidthUsagePercentage := calculatePercentage(m.NetworkLoadBytesPerSecond, m.NetworkBandwidthBytesPerSecond)
	if networkBandwidthUsagePercentage > 90 {
		leftNetworkBandwidthMb := float64(m.NetworkBandwidthBytesPerSecond-m.NetworkLoadBytesPerSecond) / (1024 * 1024) * 8
		fmt.Printf("Network bandwidth usage high: %v Mbit/s available\n", leftNetworkBandwidthMb)
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

	//if resp.Header.Get("Content-Type") != "text/plain" {
	//	return m, errors.New("Content-Type mismatch")
	//}

	reader := bufio.NewReader(resp.Body)
	buffer := make([]byte, 1024)

	n, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		return m, fmt.Errorf("error reading")
	}

	values := strings.Split(string(buffer[:n]), ",")
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
	if m.RAMTotalBytes, err = strconv.Atoi(values[1]); err != nil {
		return fmt.Errorf("RamTotalBytes parsing error: %w", err)
	}
	if m.RAMUsageBytes, err = strconv.Atoi(values[2]); err != nil {
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

func calculatePercentage(used, total int) int {
	if total == 0 {
		return 0
	}
	return int(math.Floor(float64(used) / float64(total) * 100))
}
