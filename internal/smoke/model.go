package smoke

// SmokeResult is the top-level result for a smoke test run.
type SmokeResult struct {
	Name    string        `json:"name"`
	Passed  bool          `json:"passed"`
	Total   int           `json:"total"`
	Passed_ int           `json:"passed_count"`
	Failed  int           `json:"failed_count"`
	Checks  []CheckResult `json:"checks"`
	Summary string        `json:"summary"`
}

// CheckResult represents a single check within a smoke test.
type CheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // pass | fail | skip | warn
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// GoldenPathResult holds the results of a golden path E2E smoke test.
type GoldenPathResult struct {
	BootstrapOK  bool `json:"bootstrap_ok"`
	DoctorOK     bool `json:"doctor_ok"`
	DBOK         bool `json:"db_ok"`
	ListenersOK  bool `json:"listeners_ok"`
	ConfigOK     bool `json:"config_ok"`
	AllOK        bool `json:"all_ok"`
	CheckCount   int  `json:"check_count"`
	PassCount    int  `json:"pass_count"`
}

// ProviderSmokeResult holds provider smoke check results.
type ProviderSmokeResult struct {
	HAProxyOK   bool   `json:"haproxy_ok"`
	HAProxyMsg  string `json:"haproxy_msg,omitempty"`
	CaddyOK     bool   `json:"caddy_ok"`
	CaddyMsg    string `json:"caddy_msg,omitempty"`
	AllHealthy  bool   `json:"all_healthy"`
}

// TraceVerifyResult holds trace verification results.
type TraceVerifyResult struct {
	Domain         string `json:"domain"`
	TraceOK        bool   `json:"trace_ok"`
	TraceStatus    string `json:"trace_status"`
	StepCount      int    `json:"step_count"`
	TargetMatch    bool   `json:"target_match"`
	ConnectivityOK bool   `json:"connectivity_ok"`
	ErrorCode      string `json:"error_code,omitempty"`
	Message        string `json:"message,omitempty"`
}

// FailureMatrixResult holds results for the failure matrix check.
type FailureMatrixResult struct {
	TotalCases     int              `json:"total_cases"`
	PassedCases    int              `json:"passed_cases"`
	FailedCases    int              `json:"failed_cases"`
	Cases          []FailureCase    `json:"cases"`
}

// FailureCase represents a single failure injection test.
type FailureCase struct {
	Name          string `json:"name"`
	Category      string `json:"category"` // provider | target | auth | apply | gateway
	ExpectedCode  string `json:"expected_code"`
	ActualCode    string `json:"actual_code,omitempty"`
	HTTPStatus    int    `json:"http_status"`
	LogPresent    bool   `json:"log_present"`
	TracePresent  bool   `json:"trace_present,omitempty"`
	Passed        bool   `json:"passed"`
	Message       string `json:"message,omitempty"`
}

// RestartCheckResult holds restart safety check results.
type RestartCheckResult struct {
	ConfigIntact       bool   `json:"config_intact"`
	DBIntact           bool   `json:"db_intact"`
	StateVersionStable bool   `json:"state_version_stable"`
	PendingApplyClean  bool   `json:"pending_apply_clean"`
	NoDupResources     bool   `json:"no_dup_resources"`
	Message            string `json:"message,omitempty"`
}
