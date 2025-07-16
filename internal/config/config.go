package config

import "time"

// Config представляет основную конфигурацию ShiwaTime
type Config struct {
	ShiwaTime ShiwaTimeConfig `yaml:"shiwatime"`
	Output    OutputConfig    `yaml:"output"`
	Setup     SetupConfig     `yaml:"setup"`
	Name      string          `yaml:"name,omitempty"`
	Tags      []string        `yaml:"tags,omitempty"`
	Fields    map[string]interface{} `yaml:"fields,omitempty"`
}

// ShiwaTimeConfig содержит основные настройки синхронизации времени
type ShiwaTimeConfig struct {
	License    LicenseConfig    `yaml:"license"`
	Config     ConfigPaths      `yaml:"config"`
	ClockSync  ClockSyncConfig  `yaml:"clock_sync"`
	PTPTuning  PTPTuningConfig  `yaml:"ptp_tuning"`
	SyncRTC    SyncRTCConfig    `yaml:"synchronise_rtc"`
	PTPSquared PTPSquaredConfig `yaml:"ptpsquared"`
	CLI        CLIConfig        `yaml:"cli"`
	HTTP       HTTPConfig       `yaml:"http"`
	Logging    LoggingConfig    `yaml:"logging"`
}

// LicenseConfig настройки лицензии
type LicenseConfig struct {
	KeyFile string `yaml:"keyfile"`
}

// ConfigPaths пути к дополнительным конфигурационным файлам
type ConfigPaths struct {
	PeerIDs string `yaml:"peerids"`
}

// ClockSyncConfig конфигурация синхронизации часов
type ClockSyncConfig struct {
	AdjustClock      bool              `yaml:"adjust_clock"`
	StepLimit        string            `yaml:"step_limit"`
	PrimaryClocks    []TimeSourceConfig `yaml:"primary_clocks"`
	SecondaryClocks  []TimeSourceConfig `yaml:"secondary_clocks"`
}

// TimeSourceConfig конфигурация источника времени
type TimeSourceConfig struct {
	Type       string `yaml:"type" json:"type"`
	Host       string `yaml:"host" json:"host"`
	Port       int    `yaml:"port" json:"port"`
	Interface  string `yaml:"interface" json:"interface"`
	Device     string `yaml:"device" json:"device"`
	Weight     int    `yaml:"weight" json:"weight"`
	
	// PTP-specific fields
	Domain         int    `yaml:"domain" json:"domain"`
	Profile        string `yaml:"profile" json:"profile"`
	TransportType  string `yaml:"transport_type" json:"transport_type"`
	NetworkTransport string `yaml:"network_transport" json:"network_transport"`
	ClockClass     int    `yaml:"clock_class" json:"clock_class"`
	Priority1      int    `yaml:"priority1" json:"priority1"`
	Priority2      int    `yaml:"priority2" json:"priority2"`
	LogAnnounceInterval   int `yaml:"log_announce_interval" json:"log_announce_interval"`
	LogSyncInterval       int `yaml:"log_sync_interval" json:"log_sync_interval"`
	LogDelayReqInterval   int `yaml:"log_delay_req_interval" json:"log_delay_req_interval"`
	
	// PPS-specific fields
	PPSMode       string `yaml:"pps_mode" json:"pps_mode"`
	GPIOPin       int    `yaml:"gpio_pin" json:"gpio_pin"`
	PPSKernel     bool   `yaml:"pps_kernel" json:"pps_kernel"`
	PPSAssert     bool   `yaml:"pps_assert" json:"pps_assert"`
	PPSClear      bool   `yaml:"pps_clear" json:"pps_clear"`
	
	// PHC-specific fields
	PHCIndex      int    `yaml:"phc_index" json:"phc_index"`
	PHCDevice     string `yaml:"phc_device" json:"phc_device"`
	
	// NMEA-specific fields
	BaudRate      int    `yaml:"baud_rate" json:"baud_rate"`
	DataBits      int    `yaml:"data_bits" json:"data_bits"`
	StopBits      int    `yaml:"stop_bits" json:"stop_bits"`
	Parity        string `yaml:"parity" json:"parity"`
	
	// Timecard-specific fields
	TimecardType  string `yaml:"timecard_type" json:"timecard_type"`
	Refclock      string `yaml:"refclock" json:"refclock"`
	
	// Polling configuration
	PollingInterval time.Duration `yaml:"polling_interval" json:"polling_interval"`
	PollingBurst    int           `yaml:"polling_burst" json:"polling_burst"`
	
	// Quality thresholds
	MaxOffset     time.Duration `yaml:"max_offset" json:"max_offset"`
	MaxDelay      time.Duration `yaml:"max_delay" json:"max_delay"`
	MaxJitter     time.Duration `yaml:"max_jitter" json:"max_jitter"`
	
	// Advanced options
	Trust        bool              `yaml:"trust" json:"trust"`
	PreferKernel bool              `yaml:"prefer_kernel" json:"prefer_kernel"`
	TOS          int               `yaml:"tos" json:"tos"`
	TTL          int               `yaml:"ttl" json:"ttl"`
	Options      map[string]string `yaml:"options" json:"options"`
}

// ClockConfig конфигурация системных часов  
type ClockConfig struct {
	Algorithm     string        `yaml:"algorithm" json:"algorithm"`
	Disciplining  string        `yaml:"disciplining" json:"disciplining"`
	
	// Source selection parameters
	PrimarySource   string `yaml:"primary_source" json:"primary_source"`
	SecondarySource string `yaml:"secondary_source" json:"secondary_source"`
	FallbackSource  string `yaml:"fallback_source" json:"fallback_source"`
	
	// Clock adjustment parameters
	MaxAdjustment   time.Duration `yaml:"max_adjustment" json:"max_adjustment"`
	StepThreshold   time.Duration `yaml:"step_threshold" json:"step_threshold"`
	PanicThreshold  time.Duration `yaml:"panic_threshold" json:"panic_threshold"`
	
	// PID controller parameters (for advanced clock control)
	KP            float64 `yaml:"kp" json:"kp"`               // Proportional gain
	KI            float64 `yaml:"ki" json:"ki"`               // Integral gain  
	KD            float64 `yaml:"kd" json:"kd"`               // Derivative gain
	Integrator    float64 `yaml:"integrator" json:"integrator"` // Integrator limit
	
	// Statistics and filtering
	StatisticsLength int           `yaml:"statistics_length" json:"statistics_length"`
	FilterLength     int           `yaml:"filter_length" json:"filter_length"`
	SigmaThreshold   float64       `yaml:"sigma_threshold" json:"sigma_threshold"`
	RhoThreshold     float64       `yaml:"rho_threshold" json:"rho_threshold"`
	
	// Kernel discipline
	KernelSync       bool `yaml:"kernel_sync" json:"kernel_sync"`
	KernelPPS        bool `yaml:"kernel_pps" json:"kernel_pps"`
	SyncToHWClock    bool `yaml:"sync_to_hw_clock" json:"sync_to_hw_clock"`
	
	// Hardware timestamping
	HWTimestamping   bool `yaml:"hw_timestamping" json:"hw_timestamping"`
	TimestampAll     bool `yaml:"timestamp_all" json:"timestamp_all"`
	
	// Leap second handling
	LeapSecFile      string `yaml:"leapsecfile" json:"leapsecfile"`
	LeapSecMode      string `yaml:"leapsec_mode" json:"leapsec_mode"`
	LeapSmearLength  time.Duration `yaml:"leap_smear_length" json:"leap_smear_length"`
}

// PTPTuningConfig настройки тонкой настройки PTP
type PTPTuningConfig struct {
	EnableGlobalSockets   bool                `yaml:"enable_ptp_global_sockets,omitempty"`
	RelaxDelayRequests    bool                `yaml:"relax_delay_requests,omitempty"`
	AutoDiscoverEnabled   bool                `yaml:"auto_discover_enabled,omitempty"`
	MulticastTTL          int                 `yaml:"multicast_ttl,omitempty"`
	DSCP                  DSCPConfig          `yaml:"dscp,omitempty"`
	SynchronizeTX         []string            `yaml:"synchronise_tx,omitempty"`
	PTPStandard           string              `yaml:"ptp_standard,omitempty"`
	ClockQuality          ClockQualityConfig  `yaml:"clock_quality,omitempty"`
	PHC                   PHCConfig           `yaml:"phc,omitempty"`
}

// DSCPConfig настройки DSCP для PTP сообщений
type DSCPConfig struct {
	General string `yaml:"general,omitempty"`
	Event   string `yaml:"event,omitempty"`
}

// ClockQualityConfig настройки качества часов
type ClockQualityConfig struct {
	Auto      bool   `yaml:"auto,omitempty"`
	Class     int    `yaml:"class,omitempty"`
	Accuracy  string `yaml:"accuracy,omitempty"`
	Variance  string `yaml:"variance,omitempty"`
	TimeSource string `yaml:"timesource,omitempty"`
}

// PHCConfig настройки Precision Hardware Clock
type PHCConfig struct {
	OffsetStrategy    []string `yaml:"phc_offset_strategy,omitempty"`
	LocalPref         []string `yaml:"phc_local_pref,omitempty"`
	SmoothingStrategy []string `yaml:"phc_smoothing_strategy,omitempty"`
	LPFilterEnabled   bool     `yaml:"phc_lp_filter_enabled,omitempty"`
	NGFilterEnabled   bool     `yaml:"phc_ng_filter_enabled,omitempty"`
	Samples           []string `yaml:"phc_samples,omitempty"`
	OneStep           []string `yaml:"phc_one_step,omitempty"`
	TAIOffset         string   `yaml:"tai_offset,omitempty"`
	Offsets           []string `yaml:"phc_offsets,omitempty"`
	PPSConfig         []string `yaml:"pps_config,omitempty"`
}

// SyncRTCConfig настройки синхронизации RTC
type SyncRTCConfig struct {
	Enable        bool   `yaml:"enable,omitempty"`
	ClockInterval string `yaml:"clock_interval,omitempty"`
}

// CLIConfig настройки CLI интерфейса
type CLIConfig struct {
	Enable         bool   `yaml:"enable,omitempty"`
	BindPort       int    `yaml:"bind_port,omitempty"`
	BindHost       string `yaml:"bind_host,omitempty"`
	ServerKey      string `yaml:"server_key,omitempty"`
	AuthorizedKeys string `yaml:"authorised_keys,omitempty"`
	Username       string `yaml:"username,omitempty"`
	Password       string `yaml:"password,omitempty"`
}

// HTTPConfig настройки HTTP интерфейса
type HTTPConfig struct {
	Enable   bool   `yaml:"enable,omitempty"`
	BindPort int    `yaml:"bind_port,omitempty"`
	BindHost string `yaml:"bind_host,omitempty"`
}

// LoggingConfig настройки логирования
type LoggingConfig struct {
	BufferSize   int           `yaml:"buffer_size,omitempty"`
	Stdout       StdoutConfig  `yaml:"stdout,omitempty"`
	Syslog       SyslogConfig  `yaml:"syslog,omitempty"`
}

// StdoutConfig настройки вывода в stdout
type StdoutConfig struct {
	Enable bool `yaml:"enable,omitempty"`
}

// SyslogConfig настройки syslog
type SyslogConfig struct {
	Enable bool   `yaml:"enable,omitempty"`
	Host   string `yaml:"host,omitempty"`
	Port   int    `yaml:"port,omitempty"`
}

// OutputConfig конфигурация выходных данных (Elasticsearch)
type OutputConfig struct {
	// Type определяет тип клиента метрик. Возможные значения: "native" (bulk HTTP), "beats" (Elastic Beats)
	Type string `yaml:"type,omitempty"`

	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch,omitempty"`

	// Параметры для beats-клиента (используются, если Type == "beats")
	Beats BeatsConfig `yaml:"beats,omitempty"`
}

// BeatsConfig настройки для интеграции через Elastic Beats publisher
type BeatsConfig struct {
	Hosts    []string `yaml:"hosts"`        // список адресов output (Logstash или Elasticsearch)
	Username string   `yaml:"username,omitempty"`
	Password string   `yaml:"password,omitempty"`
	APIKey   string   `yaml:"api_key,omitempty"`
	Protocol string   `yaml:"protocol,omitempty"` // http / https
}

// ElasticsearchConfig настройки Elasticsearch
type ElasticsearchConfig struct {
	Hosts                []string `yaml:"hosts"`
	Protocol             string   `yaml:"protocol,omitempty"`
	APIKey               string   `yaml:"api_key,omitempty"`
	Username             string   `yaml:"username,omitempty"`
	Password             string   `yaml:"password,omitempty"`
	CertificateAuthorities []string `yaml:"ssl.certificate_authorities,omitempty"`
	Certificate          string   `yaml:"ssl.certificate,omitempty"`
	Key                  string   `yaml:"ssl.key,omitempty"`
	VerificationMode     string   `yaml:"ssl.verification_mode,omitempty"`
}

// SetupConfig настройки установки/setup
type SetupConfig struct {
	Dashboards DashboardsConfig `yaml:"dashboards"`
	ILM        ILMConfig        `yaml:"ilm"`
}

// DashboardsConfig настройки дашбордов
type DashboardsConfig struct {
	Enabled   bool   `yaml:"enabled"`
	URL       string `yaml:"url,omitempty"`
	Directory string `yaml:"directory,omitempty"`
}

// ILMConfig настройки Index Lifecycle Management
type ILMConfig struct {
	Enabled     bool   `yaml:"enabled"`
	PolicyName  string `yaml:"policy_name,omitempty"`
	PolicyFile  string `yaml:"policy_file,omitempty"`
	CheckExists bool   `yaml:"check_exists,omitempty"`
}

// PTPSquaredConfig конфигурация PTP+Squared
type PTPSquaredConfig struct {
	Enable bool `yaml:"enable,omitempty"`
	
	// Discovery settings
	Discovery DiscoveryConfig `yaml:"discovery,omitempty"`
	
	// Key management
	KeyPath string `yaml:"keypath,omitempty"`
	
	// Network settings
	Domains  []int  `yaml:"domains,omitempty"`
	Interface string `yaml:"interface,omitempty"`
	
	// Capacity management
	SeatsToOffer      int `yaml:"seats_to_offer,omitempty"`
	SeatsToFill       int `yaml:"seats_to_fill,omitempty"`
	ConcurrentSources int `yaml:"concurrent_sources,omitempty"`
	
	// Timing intervals
	ActiveSyncInterval         int `yaml:"active_sync_interval,omitempty"`
	ActiveDelayRequestInterval int `yaml:"active_delayrequest_interval,omitempty"`
	MonitorSyncInterval        int `yaml:"monitor_sync_interval,omitempty"`
	MonitorDelayRequestInterval int `yaml:"monitor_delayrequest_interval,omitempty"`
	
	// Quality and preferences
	Capabilities     []string `yaml:"capabilities,omitempty"`
	PreferenceScore int      `yaml:"preference_score,omitempty"`
	Reservations    []string `yaml:"reservations,omitempty"`
	
	// Debug settings
	Debug bool `yaml:"debug,omitempty"`
	
	// Advanced settings
	Advanced PTPSquaredAdvancedConfig `yaml:"advanced,omitempty"`
}

// DiscoveryConfig настройки обнаружения узлов
type DiscoveryConfig struct {
	MDNS        bool     `yaml:"mdns,omitempty"`
	DHT         bool     `yaml:"dht,omitempty"`
	DHTSeedList []string `yaml:"dht_seed_list,omitempty"`
}

// PTPSquaredAdvancedConfig расширенные настройки PTP+Squared
type PTPSquaredAdvancedConfig struct {
	AsymmetryCompensation float64 `yaml:"asymmetry_compensation,omitempty"`
	IsBetterFactor        float64 `yaml:"is_better_factor,omitempty"`
	EOSWeight            float64 `yaml:"eos_weight,omitempty"`
	BaseHopCost          float64 `yaml:"base_hop_cost,omitempty"`
	SWTSCost             float64 `yaml:"swts_cost,omitempty"`
	HWTSCost             float64 `yaml:"hwts_cost,omitempty"`
	LatencyAnalysisEnable bool   `yaml:"latency_analysis_enable,omitempty"`
}