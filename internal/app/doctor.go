package app

import "context"

// DoctorReport is the aggregated result of all checks.
type DoctorReport struct {
	Checks  []CheckResult `json:"checks"`
	Summary DoctorSummary `json:"summary"`
}

// DoctorSummary counts results by severity.
type DoctorSummary struct {
	Pass int `json:"pass"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
	Skip int `json:"skip"`
}

// allChecks is the full ordered set the runner executes.
func allChecks() []func(*App) CheckResult {
	return []func(*App) CheckResult{
		checkDockerDaemon,
		checkMailpitContainer,
		checkMailpitAPI,
		checkPortSMTP,
		checkPortUI,
	}
}

// Doctor runs every check and returns the aggregated report.
func (a *App) Doctor(_ context.Context) (DoctorReport, error) {
	rep := DoctorReport{}
	for _, check := range allChecks() {
		r := check(a)
		rep.Checks = append(rep.Checks, r)
		switch r.Status {
		case SeverityPass:
			rep.Summary.Pass++
		case SeverityWarn:
			rep.Summary.Warn++
		case SeverityFail:
			rep.Summary.Fail++
		case SeveritySkip:
			rep.Summary.Skip++
		}
	}
	return rep, nil
}
