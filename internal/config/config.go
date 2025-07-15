package config

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
	Protocol              string            `yaml:"protocol"`
	Disable               bool              `yaml:"disable,omitempty"`
	MonitorOnly           bool              `yaml:"monitor_only,omitempty"`
	
	// PTP специфичные настройки
	Domain                int               `yaml:"domain,omitempty"`
	Interface             string            `yaml:"interface,omitempty"`
	ServeUnicast          bool              `yaml:"serve_unicast,omitempty"`
	ServeMulticast        bool              `yaml:"serve_multicast,omitempty"`
	ServerOnly            bool              `yaml:"server_only,omitempty"`
	AnnounceInterval      int               `yaml:"announce_interval,omitempty"`
	SyncInterval          int               `yaml:"sync_interval,omitempty"`
	DelayRequestInterval  int               `yaml:"delayrequest_interval,omitempty"`
	UnicastMasterTable    []string          `yaml:"unicast_master_table,omitempty"`
	DelayStrategy         string            `yaml:"delay_strategy,omitempty"`
	HybridE2E             bool              `yaml:"hybrid_e2e,omitempty"`
	Priority1             int               `yaml:"priority1,omitempty"`
	Priority2             int               `yaml:"priority2,omitempty"`
	UseLayer2             bool              `yaml:"use_layer2,omitempty"`
	Group                 string            `yaml:"group,omitempty"`
	Profile               string            `yaml:"profile,omitempty"`
	LogSource             string            `yaml:"logsource,omitempty"`
	AsymmetryCompensation int64             `yaml:"asymmetry_compensation,omitempty"`
	MaxPacketsPerSecond   int               `yaml:"max_packets_per_second,omitempty"`
	PeerID                string            `yaml:"peer_id,omitempty"`
	
	// NTP специфичные настройки
	IP                    string            `yaml:"ip,omitempty"`
	PollInterval          string            `yaml:"pollinterval,omitempty"`
	
	// PPS специфичные настройки
	Pin                   int               `yaml:"pin,omitempty"`
	Index                 int               `yaml:"index,omitempty"`
	CableDelay            int64             `yaml:"cable_delay,omitempty"`
	EdgeMode              string            `yaml:"edge_mode,omitempty"`
	Atomic                bool              `yaml:"atomic,omitempty"`
	LinkedDevice          string            `yaml:"linked_device,omitempty"`
	
	// NMEA специфичные настройки
	Device                string            `yaml:"device,omitempty"`
	Baud                  int               `yaml:"baud,omitempty"`
	Offset                int64             `yaml:"offset,omitempty"`
	
	// Timecard специфичные настройки
	CardConfig            []string          `yaml:"card_config,omitempty"`
	OscillatorType        string            `yaml:"oscillator_type,omitempty"`
	OCPDevice             int               `yaml:"ocp_device,omitempty"`
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
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
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