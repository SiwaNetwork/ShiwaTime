package config

import (
    "io/ioutil"
    "time"

    "gopkg.in/yaml.v3"
)

// Config is root of ShiwaTime configuration, mirroring subset of timebeat.yml
// YAML tags follow the original hierarchy where possible.
type Config struct {
    Timebeat           Timebeat            `yaml:"timebeat"`
    OutputElastic      ElasticsearchOutput `yaml:"output.elasticsearch"`
}

type Timebeat struct {
    LicenseKeyFile string      `yaml:"license.keyfile"`
    ClockSync      ClockSync   `yaml:"clock_sync"`
}

type ClockSync struct {
    AdjustClock  bool          `yaml:"adjust_clock"`
    StepLimit    Duration      `yaml:"step_limit"`
    Primary      []Source      `yaml:"primary_clocks"`
    Secondary    []Source      `yaml:"secondary_clocks"`
}

type Source struct {
    Protocol     string   `yaml:"protocol"`
    IP           string   `yaml:"ip"`
    Domain       int      `yaml:"domain"`
    PollInterval Duration `yaml:"pollinterval"`
    MonitorOnly  bool     `yaml:"monitor_only"`
    Disable      bool     `yaml:"disable"`
    Device       string   `yaml:"device"`
}

// Output alias removed; use OutputElastic directly

// Duration wraps time.Duration to enable YAML parsing with units.
type Duration struct {
    time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
    var s string
    if err := value.Decode(&s); err != nil {
        return err
    }
    dur, err := time.ParseDuration(s)
    if err != nil {
        return err
    }
    d.Duration = dur
    return nil
}

// Load reads YAML config from given path.
func Load(path string) (*Config, error) {
    data, err := ioutil.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}

// ElasticsearchOutput holds hosts list for Elastic.
type ElasticsearchOutput struct {
    Hosts []string `yaml:"hosts"`
}