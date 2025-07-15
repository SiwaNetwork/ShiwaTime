package config

import (
	"time"
)

// Config represents the main configuration structure for ShiwaTime
type Config struct {
	ShiwaTime ShiwaTimeConfig `yaml:"shiwatime"`
	
	// Elastic Beats configuration
	Output OutputConfig `yaml:"output"`
	Setup  SetupConfig  `yaml:"setup"`
	Logging LoggingConfig `yaml:"logging"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
}

// ShiwaTimeConfig contains all time synchronization related configuration
type ShiwaTimeConfig struct {
	License  LicenseConfig  `yaml:"license"`
	Config   GeneralConfig  `yaml:"config"`
	ClockSync ClockSyncConfig `yaml:"clock_sync"`
	Advanced AdvancedConfig `yaml:"advanced"`
}

// LicenseConfig holds license configuration
type LicenseConfig struct {
	KeyFile string `yaml:"keyfile"`
}

// GeneralConfig holds general configuration
type GeneralConfig struct {
	PeerIDs string `yaml:"peerids"`
}

// ClockSyncConfig contains clock synchronization settings
type ClockSyncConfig struct {
	AdjustClock     bool           `yaml:"adjust_clock"`
	StepLimit       string         `yaml:"step_limit"`
	PrimaryClocks   []ClockSource  `yaml:"primary_clocks"`
	SecondaryClocks []ClockSource  `yaml:"secondary_clocks"`
	PTPSquared      PTPSquaredConfig `yaml:"ptpsquared"`
	TaaS           TaaSConfig      `yaml:"taas"`
}

// ClockSource represents a time source configuration
type ClockSource struct {
	Protocol              string   `yaml:"protocol"`
	Domain                int      `yaml:"domain,omitempty"`
	IP                    string   `yaml:"ip,omitempty"`
	PollInterval          string   `yaml:"pollinterval,omitempty"`
	ServeUnicast          bool     `yaml:"serve_unicast,omitempty"`
	ServeMulticast        bool     `yaml:"serve_multicast,omitempty"`
	ServerOnly            bool     `yaml:"server_only,omitempty"`
	AnnounceInterval      int      `yaml:"announce_interval,omitempty"`
	SyncInterval          int      `yaml:"sync_interval,omitempty"`
	DelayRequestInterval  int      `yaml:"delayrequest_interval,omitempty"`
	UnicastMasterTable    []string `yaml:"unicast_master_table,omitempty"`
	DelayStrategy         string   `yaml:"delay_strategy,omitempty"`
	Priority1             int      `yaml:"priority1,omitempty"`
	Priority2             int      `yaml:"priority2,omitempty"`
	MonitorOnly           bool     `yaml:"monitor_only,omitempty"`
	Interface             string   `yaml:"interface,omitempty"`
	Profile               string   `yaml:"profile,omitempty"`
	Disable               bool     `yaml:"disable,omitempty"`
	Group                 string   `yaml:"group,omitempty"`
	LogSource             string   `yaml:"logsource,omitempty"`
	AsymmetryCompensation int      `yaml:"asymmetry_compensation,omitempty"`
	
	// PPS specific
	Pin         int    `yaml:"pin,omitempty"`
	Index       int    `yaml:"index,omitempty"`
	CableDelay  int    `yaml:"cable_delay,omitempty"`
	EdgeMode    string `yaml:"edge_mode,omitempty"`
	Atomic      bool   `yaml:"atomic,omitempty"`
	LinkedDevice string `yaml:"linked_device,omitempty"`
	
	// NMEA specific
	Device string `yaml:"device,omitempty"`
	Baud   int    `yaml:"baud,omitempty"`
	Offset int    `yaml:"offset,omitempty"`
	
	// PHC specific
	// Device field is reused from NMEA
	
	// Timecard specific
	CardConfig   []string `yaml:"card_config,omitempty"`
	OCPDevice    int      `yaml:"ocp_device,omitempty"`
	OscillatorType string `yaml:"oscillator_type,omitempty"`
}

// PTPSquaredConfig represents PTPSquared configuration
type PTPSquaredConfig struct {
	Enable bool `yaml:"enable"`
	// Add more fields as needed
}

// TaaSConfig represents Time as a Service configuration
type TaaSConfig struct {
	Enable bool `yaml:"enable"`
	// Add more fields as needed
}

// AdvancedConfig contains advanced configuration options
type AdvancedConfig struct {
	Steering           SteeringConfig           `yaml:"steering"`
	InterferenceMonitor InterferenceMonitorConfig `yaml:"interference_monitor"`
	ExtendedStepLimits ExtendedStepLimitsConfig  `yaml:"extended_step_limits"`
	WindowsSpecific    WindowsSpecificConfig     `yaml:"windows_specific"`
	LinuxSpecific      LinuxSpecificConfig       `yaml:"linux_specific"`
	PTPTuning         PTPTuningConfig          `yaml:"ptp_tuning"`
	ClockQuality      ClockQualityConfig       `yaml:"clock_quality"`
	SynchroniseRTC    SynchroniseRTCConfig     `yaml:"synchronise_rtc"`
	CLI               CLIConfig                `yaml:"cli"`
	HTTP              HTTPConfig               `yaml:"http"`
	Logging           InternalLoggingConfig    `yaml:"logging"`
}

// SteeringConfig contains steering algorithm configuration
type SteeringConfig struct {
	Algo                      string `yaml:"algo"`
	AlgoLogging               bool   `yaml:"algo_logging"`
	OutlierFilterEnabled      bool   `yaml:"outlier_filter_enabled"`
	OutlierFilterType         string `yaml:"outlier_filter_type"`
	ServoOffsetArrivalDriven  bool   `yaml:"servo_offset_arrival_driven"`
}

// InterferenceMonitorConfig for clock interference monitoring
type InterferenceMonitorConfig struct {
	BackoffTimer string `yaml:"backoff_timer"`
}

// ExtendedStepLimitsConfig for extended step limits
type ExtendedStepLimitsConfig struct {
	Forward  StepLimitConfig `yaml:"forward"`
	Backward StepLimitConfig `yaml:"backward"`
}

// StepLimitConfig for individual step limit configuration
type StepLimitConfig struct {
	Boundary string `yaml:"boundary"`
	Limit    string `yaml:"limit"`
}

// WindowsSpecificConfig for Windows-specific settings
type WindowsSpecificConfig struct {
	DisableOSRelax bool `yaml:"disable_os_relax"`
}

// LinuxSpecificConfig for Linux-specific settings
type LinuxSpecificConfig struct {
	HardwareTimestamping        bool     `yaml:"hardware_timestamping"`
	ExternalSoftwareTimestamping bool     `yaml:"external_software_timestamping"`
	SyncNICSlaves               bool     `yaml:"sync_nic_slaves"`
	DisableAdjustment           []string `yaml:"disable_adjustment"`
	PHCOffsetStrategy           []string `yaml:"phc_offset_strategy"`
	PHCLocalPref                []string `yaml:"phc_local_pref"`
	PHCSmoothingStrategy        []string `yaml:"phc_smoothing_strategy"`
	PHCLPFilterEnabled          bool     `yaml:"phc_lp_filter_enabled"`
	PHCNGFilterEnabled          bool     `yaml:"phc_ng_filter_enabled"`
	PHCSamples                  []string `yaml:"phc_samples"`
	PHCOneStep                  []string `yaml:"phc_one_step"`
	TAIOffset                   string   `yaml:"tai_offset"`
	PHCOffsets                  []string `yaml:"phc_offsets"`
	PPSConfig                   []string `yaml:"pps_config"`
}

// PTPTuningConfig for PTP protocol tuning
type PTPTuningConfig struct {
	EnablePTPGlobalSockets bool            `yaml:"enable_ptp_global_sockets"`
	RelaxDelayRequests     bool            `yaml:"relax_delay_requests"`
	AutoDiscoverEnabled    bool            `yaml:"auto_discover_enabled"`
	MulticastTTL           int             `yaml:"multicast_ttl"`
	DSCP                   DSCPConfig      `yaml:"dscp"`
	SynchroniseTX          []string        `yaml:"synchronise_tx"`
	PTPStandard            string          `yaml:"ptp_standard"`
	ClockQuality           ClockQualityConfig `yaml:"clock_quality"`
}

// DSCPConfig for DSCP field configuration
type DSCPConfig struct {
	General string `yaml:"general"`
	Event   string `yaml:"event"`
}

// ClockQualityConfig for clock quality settings
type ClockQualityConfig struct {
	Auto       bool   `yaml:"auto"`
	Class      int    `yaml:"class"`
	Accuracy   string `yaml:"accuracy"`
	Variance   string `yaml:"variance"`
	TimeSource string `yaml:"timesource"`
}

// SynchroniseRTCConfig for RTC synchronization
type SynchroniseRTCConfig struct {
	Enable        bool   `yaml:"enable"`
	ClockInterval string `yaml:"clock_interval"`
}

// CLIConfig for CLI interface configuration
type CLIConfig struct {
	Enable         bool   `yaml:"enable"`
	BindPort       int    `yaml:"bind_port"`
	BindHost       string `yaml:"bind_host"`
	ServerKey      string `yaml:"server_key"`
	AuthorisedKeys string `yaml:"authorised_keys"`
	Username       string `yaml:"username"`
	Password       string `yaml:"password"`
}

// HTTPConfig for HTTP interface configuration
type HTTPConfig struct {
	Enable   bool   `yaml:"enable"`
	BindPort int    `yaml:"bind_port"`
	BindHost string `yaml:"bind_host"`
}

// InternalLoggingConfig for internal logging configuration
type InternalLoggingConfig struct {
	StdoutEnable bool   `yaml:"stdout.enable"`
	BufferSize   int    `yaml:"buffer_size"`
	SyslogEnable bool   `yaml:"syslog.enable"`
	SyslogHost   string `yaml:"syslog.host"`
	SyslogPort   int    `yaml:"syslog.port"`
}

// OutputConfig for Elasticsearch output configuration
type OutputConfig struct {
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
}

// ElasticsearchConfig for Elasticsearch settings
type ElasticsearchConfig struct {
	Hosts    []string `yaml:"hosts"`
	Protocol string   `yaml:"protocol"`
	APIKey   string   `yaml:"api_key"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
}

// SetupConfig for setup configuration
type SetupConfig struct {
	Dashboards DashboardsConfig `yaml:"dashboards"`
	ILM        ILMConfig        `yaml:"ilm"`
}

// DashboardsConfig for Kibana dashboards
type DashboardsConfig struct {
	Enabled   bool   `yaml:"enabled"`
	URL       string `yaml:"url"`
	Directory string `yaml:"directory"`
}

// ILMConfig for Index Lifecycle Management
type ILMConfig struct {
	Enabled       bool   `yaml:"enabled"`
	PolicyName    string `yaml:"policy_name"`
	PolicyFile    string `yaml:"policy_file"`
	CheckExists   bool   `yaml:"check_exists"`
	Overwrite     bool   `yaml:"overwrite"`
	RolloverAlias string `yaml:"rollover_alias"`
}

// LoggingConfig for logging configuration
type LoggingConfig struct {
	Level   string           `yaml:"level"`
	Metrics MetricsConfig    `yaml:"metrics"`
	ToFiles bool             `yaml:"to_files"`
	Files   LogFilesConfig   `yaml:"files"`
}

// MetricsConfig for metrics logging
type MetricsConfig struct {
	Enabled bool          `yaml:"enabled"`
	Period  time.Duration `yaml:"period"`
}

// LogFilesConfig for log file configuration
type LogFilesConfig struct {
	Path             string `yaml:"path"`
	Name             string `yaml:"name"`
	RotateEveryBytes int    `yaml:"rotateeverybytes"`
	KeepFiles        int    `yaml:"keepfiles"`
	Permissions      string `yaml:"permissions"`
	Interval         string `yaml:"interval"`
	RotateOnStartup  bool   `yaml:"rotateonstartup"`
}

// MonitoringConfig for monitoring configuration
type MonitoringConfig struct {
	Enabled       bool                    `yaml:"enabled"`
	ClusterUUID   string                  `yaml:"cluster_uuid"`
	Elasticsearch ElasticsearchConfig     `yaml:"elasticsearch"`
}