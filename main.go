//go:build wasip1

package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/glasslabs/client-go"
	"github.com/pawal/go-hass"
)

// Config is the module configuration.
type Config struct {
	URL       string `json:"url"`
	Token     string `json:"token"`
	SensorIDs struct {
		GeyserPct      string `json:"geyserPct"`
		TankPct        string `json:"tankPct"`
		WaterConnected string `json:"waterConnected"`
		GeyserHeating  string `json:"geyserHeating"`
	} `json:"sensorIds"`
	Geyser struct {
		Warning int `json:"warning"`
		Low     int `json:"low"`
	} `json:"geyser"`
	Tank struct {
		Warning int `json:"warning"`
		Low     int `json:"low"`
	} `json:"tank"`
}

// NewConfig creates a default configuration for the module.
func NewConfig() *Config {
	return &Config{}
}

var (
	mod *client.Module
	log *client.Logger
	cfg *Config

	mu             sync.Mutex
	geyserPct      float64
	tankPct        float64
	waterConnected = true
	geyserHeating  bool

	renderMu      sync.Mutex
	lastRender    time.Time
	renderPending bool
)

func main() {
	log = client.NewLogger()

	var err error
	mod, err = client.NewModule()
	if err != nil {
		log.Error("Could not create module", "error", err.Error())
		return
	}

	cfg = NewConfig()
	if err = mod.ParseConfig(cfg); err != nil {
		log.Error("Could not parse config", "error", err.Error())
		return
	}

	log.Info("Module ready", "module", mod.Name())

	render()

	for {
		ha := hass.NewAccess(cfg.URL, "")
		ha.SetBearerToken(cfg.Token)

		if err = ha.CheckAPI(); err != nil {
			log.Error("Could not connect to Home Assistant", "error", err.Error())
			time.Sleep(10 * time.Second)
			continue
		}

		if err = syncStates(ha); err != nil {
			log.Error("Could not sync states", "error", err.Error())
			time.Sleep(10 * time.Second)
			continue
		}

		if err := listenStates(ha); err != nil {
			log.Error("State listener error", "error", err.Error())
		}

		time.Sleep(5 * time.Second)
	}
}

func syncStates(ha *hass.Access) error {
	states, err := ha.FilterStates("sensor", "binary_sensor", "switch")
	if err != nil {
		return fmt.Errorf("getting states: %w", err)
	}

	mu.Lock()
	for _, state := range states {
		updateState(state.EntityID, state.State)
	}
	mu.Unlock()

	scheduleRender()
	return nil
}

func listenStates(ha *hass.Access) error {
	l, err := ha.ListenEvents()
	if err != nil {
		return fmt.Errorf("calling listen: %w", err)
	}
	defer func() { _ = l.Close() }()

	for {
		event, err := l.NextStateChanged()
		if err != nil {
			return fmt.Errorf("listening for event: %w", err)
		}

		if event.EventType != "state_changed" {
			continue
		}
		prefix := strings.TrimSuffix(strings.SplitAfter(event.Data.EntityID, ".")[0], ".")
		if !slices.Contains([]string{"sensor", "binary_sensor", "switch"}, prefix) {
			continue
		}

		mu.Lock()
		changed := updateState(event.Data.EntityID, event.Data.NewState.State)
		mu.Unlock()

		if changed {
			scheduleRender()
		}
	}
}

// updateState updates the internal state for the given entity.
// Returns true if a relevant entity was updated.
// mu must be held by the caller.
func updateState(id, state string) bool {
	switch id {
	case cfg.SensorIDs.GeyserPct:
		f, err := strconv.ParseFloat(state, 64)
		if err != nil {
			return false
		}
		if f > 100 {
			f = 100
		}
		geyserPct = f
	case cfg.SensorIDs.TankPct:
		f, err := strconv.ParseFloat(state, 64)
		if err != nil {
			return false
		}
		tankPct = f
	case cfg.SensorIDs.WaterConnected:
		waterConnected = state == "on"
	case cfg.SensorIDs.GeyserHeating:
		geyserHeating = state == "on"
	default:
		return false
	}
	return true
}

// arcSweep converts pct (0–100) to a clockwise sweep angle in degrees over the given span.
func arcSweep(pct, spanDegrees float64) float32 {
	return float32(pct / 100.0 * spanDegrees)
}

const renderInterval = time.Second

// scheduleRender throttles render calls to at most once per second.
// If called within the cooldown window a single deferred render is scheduled
// so the most recent state is always eventually displayed.
func scheduleRender() {
	renderMu.Lock()
	now := time.Now()
	if now.Sub(lastRender) >= renderInterval {
		lastRender = now
		renderMu.Unlock()
		render()
		return
	}
	if !renderPending {
		renderPending = true
		remaining := renderInterval - now.Sub(lastRender)
		go func() {
			time.Sleep(remaining)
			renderMu.Lock()
			renderPending = false
			lastRender = time.Now()
			renderMu.Unlock()
			render()
		}()
	}
	renderMu.Unlock()
}

const (
	// warningIconD is the SVG path d-string for the water-disconnect warning triangle.
	//nolint:lll
	warningIconD = "M 35.5246 29.8246 L 19.8005 3.7476 c -0.3974 -0.659 -1.1109 -1.062 -1.8805 -1.062 c -0.7696 0 -1.4832 0.4029 -1.8805 1.062 L 0.3154 29.8246 c -0.4089 0.6783 -0.421 1.5242 -0.0316 2.2138 c 0.3895 0.6896 1.1201 1.1161 1.9121 1.1161 h 31.4481 c 0.792 0 1.5226 -0.4265 1.9121 -1.1161 C 35.9456 31.3488 35.9335 30.5029 35.5246 29.8246 z M 22.5178 19.5906 l -5.911 8.0822 c -0.1345 0.1839 -0.379 0.2493 -0.5873 0.1571 c -0.2083 -0.0922 -0.3244 -0.317 -0.2787 -0.5403 l 1.2289 -6.0082 H 13.9384 c -0.2927 0 -0.5605 -0.1644 -0.6929 -0.4254 c -0.1324 -0.261 -0.1071 -0.5743 0.0657 -0.8105 l 5.911 -8.0823 c 0.1345 -0.1839 0.379 -0.2493 0.5873 -0.1571 c 0.2084 0.0922 0.3244 0.317 0.2787 0.5403 l -1.2289 6.0082 h 3.0313 c 0.2927 0 0.5605 0.1644 0.6929 0.4254 C 22.716 19.041 22.6906 19.3544 22.5178 19.5906 z"

	// tapIconD is the SVG path d-string for the static tap/pipe icon.
	//nolint:lll
	tapIconD = "M17 6H16V5C16 3.9 15.1 3 14 3H10C8.9 3 8 3.9 8 5V6H7C3.69 6 1 8.69 1 12S3.69 18 7 18V21H9V18H15V21H17V18C20.31 18 23 15.31 23 12S20.31 6 17 6M10 5H14V6H10V5Z"
)

func render() {
	mu.Lock()
	gp := geyserPct
	tp := tankPct
	wc := waterConnected
	gh := geyserHeating
	mu.Unlock()

	geyserColor := "#5794f2"
	switch {
	case gh:
		geyserColor = "#ff9830"
	case cfg.Geyser.Low > 0 && gp <= float64(cfg.Geyser.Low):
		geyserColor = "#f2495c"
	case (cfg.Geyser.Warning > 0 && gp <= float64(cfg.Geyser.Warning)):
		geyserColor = "#ff9830"
	}
	geyserBC := "#282828"
	if gh {
		geyserBC = "#3d2d1d"
	}

	tankColor := "#37c7ff"
	switch {
	case cfg.Tank.Low > 0 && tp <= float64(cfg.Tank.Low):
		tankColor = "#f2495c"
	case cfg.Tank.Warning > 0 && tp <= float64(cfg.Tank.Warning):
		tankColor = "#ff9830"
	}

	// Canvas is 250×250 (matching the original SVG viewBox).
	//
	// Arc spans derived from the original SVG dasharray geometry:
	//   Geyser background track:  510.82 / 2π×100 × 360 ≈ 292.65°
	//   Geyser progress max span: 0.78 × 360 = 280.8°  (starts at 129°, CW)
	//   Tank background track:    109.82 / 2π×100 × 360 ≈ 62.93°
	//   Tank progress max span:   0.16 × 360 = 57.6°   (ends at 118°, CCW)
	ops := []client.DrawOp{
		// Geyser background track.
		client.NewArc(125, 125, 100, 129, 280.8, 20, geyserBC),
		// Geyser progress ring: CW from 129°, grows with geyser %.
		client.NewArc(125, 125, 100, 129, arcSweep(gp, 280.8), 5, geyserColor),
		// Tank background track.
		client.NewArc(125, 125, 100, 118, -57.6, 15, "#1e1e1e"),
		// Tank progress ring: CCW from 118°, grows into the track as tank % rises.
		client.NewArc(125, 125, 100, 118, -arcSweep(tp, 57.6), 5, tankColor),
	}

	// Water-disconnect warning icon (conditional).
	if !wc {
		ops = append(ops, client.NewPath(112, 150, 0.8, warningIconD, "#f2495c"))
	}

	// Static tap/pipe icon.
	ops = append(ops, client.NewPath(113, 180, 1.0, tapIconD, "#aaaaaa"))

	// Centre geyser percentage label.
	ops = append(ops,
		client.NewLabel(125, 125, "middle",
			client.NewRun(strconv.Itoa(int(gp)), client.WithRunFontSize(60), client.WithRunColor("#ffffff")),
			client.NewRun("%", client.WithRunFontSize(20), client.WithRunBaselineShift(33), client.WithRunColor("#ffffff")),
		),
	)

	mod.Render(client.NewCanvas(250, 250, ops...))
}
