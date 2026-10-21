// Package probes reads DS18B20 1-Wire temperature probes through the Linux
// w1 sysfs interface (w1-gpio + w1-therm kernel drivers) and publishes the
// readings to the Infinitive cache so they reach the web UI over the websocket.
package probes

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	// CacheKey is the cache / websocket source name the readings are published under.
	CacheKey = "probes"

	defaultBasePath     = "/sys/bus/w1/devices"
	defaultPollInterval = 5 * time.Second

	// DS18B20 family code; the sysfs directory is "28-<12 hex digits>".
	familyPrefix = "28-"

	// A DS18B20 reports exactly 85.000 C when it has just powered up and has not
	// completed a conversion (a common symptom of wiring or pull-up problems).
	powerOnResetMilliC = 85000
)

// Probe is the state of a single temperature probe.
type Probe struct {
	Label string   `json:"label"`
	ID    string   `json:"id"`
	OK    bool     `json:"ok"`
	TempC *float64 `json:"tempC"`
	TempF *float64 `json:"tempF"`
	Error string   `json:"error,omitempty"`
}

// Readings is what is published to the cache and returned by the API.
type Readings struct {
	Top    Probe `json:"top"`
	Bottom Probe `json:"bottom"`
}

// Updater is satisfied by *cache.Cache.
type Updater interface {
	Update(name string, data any)
}

// Config selects which sensors are the top and bottom probes.
type Config struct {
	// TopID and BottomID are 1-Wire IDs such as "28-0316a2795e3b". If both are
	// empty the sensors found on the bus are sorted by ID and assigned in order
	// (first = top, second = bottom). Run once, note the logged IDs, and pin
	// them with flags so that swapping or replacing a probe cannot silently
	// swap the labels.
	TopID    string
	BottomID string

	BasePath     string
	PollInterval time.Duration
}

type Monitor struct {
	cfg   Config
	cache Updater

	mu   sync.RWMutex
	last Readings
}

func NewMonitor(cache Updater, cfg Config) *Monitor {
	if cfg.BasePath == "" {
		cfg.BasePath = defaultBasePath
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}
	m := &Monitor{cfg: cfg, cache: cache}
	m.last = Readings{
		Top:    Probe{Label: "Wine (top)", ID: cfg.TopID, Error: "not read yet"},
		Bottom: Probe{Label: "Wine (bottom)", ID: cfg.BottomID, Error: "not read yet"},
	}
	cache.Update(CacheKey, m.last)
	return m
}

// Get returns the most recent readings.
func (m *Monitor) Get() Readings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.last
}

// Run polls until ctx is cancelled.
func (m *Monitor) Run(ctx context.Context) {
	ticker := time.NewTicker(m.cfg.PollInterval)
	defer ticker.Stop()

	m.poll()
	for {
		select {
		case <-ticker.C:
			m.poll()
		case <-ctx.Done():
			return
		}
	}
}

func (m *Monitor) poll() {
	topID, bottomID, err := m.resolveIDs()

	r := Readings{
		Top:    Probe{Label: "Wine (top)", ID: topID},
		Bottom: Probe{Label: "Wine (bottom)", ID: bottomID},
	}

	if err != nil {
		r.Top.Error = err.Error()
		r.Bottom.Error = err.Error()
	} else {
		// Each read waits for a ~750 ms conversion, so read both at once.
		var wg sync.WaitGroup
		for _, p := range []*Probe{&r.Top, &r.Bottom} {
			wg.Add(1)
			go func(p *Probe) {
				defer wg.Done()
				m.readInto(p)
			}(p)
		}
		wg.Wait()
	}

	m.mu.Lock()
	m.last = r
	m.mu.Unlock()
	m.cache.Update(CacheKey, r)
}

// resolveIDs returns the sensor IDs to use for the top and bottom probes.
func (m *Monitor) resolveIDs() (top, bottom string, err error) {
	top, bottom = m.cfg.TopID, m.cfg.BottomID
	if top != "" && bottom != "" {
		return top, bottom, nil
	}

	found, err := m.discover()
	if err != nil {
		return top, bottom, err
	}

	// Fill in any ID that wasn't pinned, using sensors that aren't already pinned.
	var free []string
	for _, id := range found {
		if id != top && id != bottom {
			free = append(free, id)
		}
	}
	if top == "" && len(free) > 0 {
		top, free = free[0], free[1:]
	}
	if bottom == "" && len(free) > 0 {
		bottom = free[0]
	}

	if len(found) != 2 {
		log.Warnf("probes: expected 2 DS18B20 sensors on the 1-Wire bus, found %d: %v", len(found), found)
	}
	log.Debugf("probes: top=%q bottom=%q (found %v)", top, bottom, found)
	return top, bottom, nil
}

func (m *Monitor) discover() ([]string, error) {
	entries, err := os.ReadDir(m.cfg.BasePath)
	if err != nil {
		return nil, fmt.Errorf("1-Wire bus not available (is dtoverlay=w1-gpio enabled?): %w", err)
	}
	var ids []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), familyPrefix) {
			ids = append(ids, e.Name())
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func (m *Monitor) readInto(p *Probe) {
	if p.ID == "" {
		p.Error = "sensor not found"
		return
	}
	c, err := readSensor(filepath.Join(m.cfg.BasePath, p.ID, "w1_slave"))
	if err != nil {
		p.Error = err.Error()
		log.Debugf("probes: %s (%s): %v", p.Label, p.ID, err)
		return
	}
	f := round1(c*9/5 + 32)
	c = round1(c)
	p.OK, p.TempC, p.TempF = true, &c, &f
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// readSensor parses a w1_slave file, which looks like:
//
//	72 01 4b 46 7f ff 0e 10 57 : crc=57 YES
//	72 01 4b 46 7f ff 0e 10 57 t=23125
func readSensor(path string) (float64, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read failed: %w", err)
	}
	return parseW1Slave(string(raw))
}

func parseW1Slave(s string) (float64, error) {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) < 2 {
		return 0, errors.New("unexpected sensor output")
	}
	if !strings.HasSuffix(strings.TrimSpace(lines[0]), "YES") {
		return 0, errors.New("CRC check failed")
	}
	idx := strings.Index(lines[1], "t=")
	if idx < 0 {
		return 0, errors.New("no temperature in sensor output")
	}
	milliC, err := strconv.Atoi(strings.TrimSpace(lines[1][idx+2:]))
	if err != nil {
		return 0, fmt.Errorf("bad temperature value: %w", err)
	}
	if milliC == powerOnResetMilliC {
		return 0, errors.New("sensor returned power-on value (85 C); check wiring and pull-up resistor")
	}
	return float64(milliC) / 1000, nil
}
