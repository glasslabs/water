//go:build js && wasm

package main

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/glasslabs/client-go"
	"github.com/pawal/go-hass"
)

var (
	//go:embed assets/style.css
	css []byte

	//go:embed assets/index.html
	html []byte
)

// Config is the module configuration.
type Config struct {
	URL       string `yaml:"url"`
	Token     string `yaml:"token"`
	SensorIDs struct {
		GeyserPct string `yaml:"geyserPct"`
		TankPct   string `yaml:"tankPct"`
	} `yaml:"sensorIds"`
	Geyser struct {
		Warning int `yaml:"warning"`
		Low     int `yaml:"low"`
	} `yaml:"geyser"`
	Tank struct {
		Warning int `yaml:"warning"`
		Low     int `yaml:"low"`
	} `yaml:"tank"`
}

// NewConfig creates a default configuration for the module.
func NewConfig() *Config {
	return &Config{}
}

func main() {
	log := client.NewLogger()
	mod, err := client.NewModule()
	if err != nil {
		log.Error("Could not create module", "error", err.Error())
		return
	}

	cfg := NewConfig()
	if err = mod.ParseConfig(&cfg); err != nil {
		log.Error("Could not parse config", "error", err.Error())
		return
	}

	log.Info("Loading Module", "module", mod.Name())

	m := &Module{
		mod: mod,
		cfg: cfg,
		log: log,
	}

	if err = m.setup(); err != nil {
		log.Error("Could not setup module", "error", err.Error())
		return
	}

	first := true
	for {
		if !first {
			time.Sleep(10 * time.Second)
		}
		first = false

		if err = m.syncStates(); err != nil {
			log.Error("Could not sync states", "error", err.Error())
			continue
		}

		if err = m.listenStates(); err != nil {
			log.Error("Could not listen to states", "error", err.Error())
			continue
		}
	}
}

// Module runs the module.
type Module struct {
	mod *client.Module
	cfg *Config

	ha *hass.Access

	log *client.Logger
}

func (m *Module) setup() error {
	if err := m.mod.LoadCSS(string(css)); err != nil {
		return fmt.Errorf("loading css: %w", err)
	}
	m.mod.Element().SetInnerHTML(string(html))

	ha := hass.NewAccess(m.cfg.URL, "")
	ha.SetBearerToken(m.cfg.Token)
	if err := ha.CheckAPI(); err != nil {
		return fmt.Errorf("could not connect to home assistant: %w", err)
	}
	m.ha = ha

	return nil
}

func (m *Module) syncStates() error {
	states, err := m.ha.FilterStates("sensor")
	if err != nil {
		return fmt.Errorf("getting states: %w", err)
	}

	for _, state := range states {
		m.updateState(state.EntityID, state.State)
	}
	return nil
}

func (m *Module) listenStates() error {
	l, err := m.ha.ListenEvents()
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
		if strings.TrimSuffix(strings.SplitAfter(event.Data.EntityID, ".")[0], ".") != "sensor" {
			continue
		}

		m.updateState(event.Data.EntityID, event.Data.NewState.State)
	}
}

const percentageVar = "--percentage: "

func (m *Module) updateState(id, state string) {
	switch id {
	case m.cfg.SensorIDs.GeyserPct:
		per, err := strconv.ParseFloat(state, 64)
		if err != nil {
			return
		}
		if per > 100 {
			per = 100
		}
		perStr := strconv.FormatFloat(per, 'f', 0, 64)

		var class string
		if per <= float64(m.cfg.Geyser.Low) {
			class = "low"
		} else if per <= float64(m.cfg.Geyser.Warning) {
			class = "warning"
		}

		if elem := m.mod.Element().QuerySelector("#heat"); elem != nil {
			elem.SetAttribute("style", percentageVar+perStr)
			elem.Class().Remove("low")
			elem.Class().Remove("warning")
			if class != "" {
				elem.Class().Add(class)
			}
		}
		if elem := m.mod.Element().QuerySelector("#geyserText .super"); elem != nil {
			elem.SetTextContent(strconv.Itoa(int(per)))
		}
	case m.cfg.SensorIDs.TankPct:
		per, err := strconv.ParseFloat(state, 64)
		if err != nil {
			return
		}
		perStr := strconv.FormatFloat(per, 'f', 2, 64)

		var class string
		if per <= float64(m.cfg.Tank.Low) {
			class = "low"
		} else if per <= float64(m.cfg.Tank.Warning) {
			class = "warning"
		}

		if elem := m.mod.Element().QuerySelector("#water"); elem != nil {
			elem.SetAttribute("style", percentageVar+perStr)
			elem.Class().Remove("low")
			elem.Class().Remove("warning")
			if class != "" {
				elem.Class().Add(class)
			}
		}
	}
}
