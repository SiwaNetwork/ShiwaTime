package service

import (
	"fmt"

	"github.com/shiwatime/shiwatime/pkg/clock"
	"github.com/shiwatime/shiwatime/pkg/config"
	"github.com/shiwatime/shiwatime/pkg/ntp"
	// "github.com/shiwatime/shiwatime/pkg/ptp"
	// Add other protocol imports as implemented
)

// createTimeSource creates a time source based on configuration
func createTimeSource(cfg config.ClockSource, priority int) (clock.TimeSource, error) {
	switch cfg.Protocol {
	case "ntp":
		return ntp.NewNTPSource(cfg, priority)
	
	// case "ptp":
	//     return ptp.NewPTPSource(cfg, priority)
	
	// case "pps":
	//     return pps.NewPPSSource(cfg, priority)
	
	// case "nmea":
	//     return nmea.NewNMEASource(cfg, priority)
	
	// case "phc":
	//     return phc.NewPHCSource(cfg, priority)
	
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", cfg.Protocol)
	}
}