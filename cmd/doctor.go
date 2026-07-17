package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

var doctorFlags struct {
	asJSON bool
	fix    bool
	yes    bool
}

func newDoctorCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose mailpitter prerequisites; --fix to remediate",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if doctorFlags.yes && !doctorFlags.fix {
				return app.InvalidArgsf("--yes requires --fix")
			}
			if doctorFlags.asJSON && doctorFlags.fix {
				return app.InvalidArgsf("--json cannot be combined with --fix")
			}
			a := app.New()
			rep, err := a.Doctor(cmd.Context())
			if err != nil {
				return err
			}
			if doctorFlags.fix {
				return runFixLoop(cmd, rep, doctorFlags.yes)
			}
			if doctorFlags.asJSON {
				renderDoctorJSON(cmd.OutOrStdout(), rep)
			} else {
				renderDoctorHuman(cmd.OutOrStdout(), rep)
			}
			if rep.Summary.Fail > 0 {
				return fmt.Errorf("doctor: %d failing check(s)", rep.Summary.Fail)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&doctorFlags.asJSON, "json", false, "Output as JSON")
	c.Flags().BoolVar(&doctorFlags.fix, "fix", false, "Apply available fixes (interactive)")
	c.Flags().BoolVar(&doctorFlags.yes, "yes", false, "Apply every fix without prompting (requires --fix)")
	return c
}

func renderDoctorHuman(w io.Writer, rep app.DoctorReport) {
	for _, c := range rep.Checks {
		_, _ = fmt.Fprintf(w, "[%s] %-20s %s\n", c.Status, c.Name, c.Summary)
		for _, d := range c.Detail {
			_, _ = fmt.Fprintf(w, "       %s\n", d)
		}
		if c.Fix != nil {
			_, _ = fmt.Fprintf(w, "       fix: %s\n", c.Fix.Label)
		}
	}
	_, _ = fmt.Fprintln(w)
	issues := rep.Summary.Fail + rep.Summary.Warn
	if issues == 0 {
		_, _ = fmt.Fprintln(w, "all checks passed.")
		return
	}
	_, _ = fmt.Fprintf(w, "%d issue(s) found (%d FAIL, %d WARN). Run with --fix to remediate.\n",
		issues, rep.Summary.Fail, rep.Summary.Warn)
}

func renderDoctorJSON(w io.Writer, rep app.DoctorReport) {
	out, _ := json.MarshalIndent(rep, "", "  ")
	_, _ = fmt.Fprintln(w, string(out))
}

func runFixLoop(cmd *cobra.Command, rep app.DoctorReport, yes bool) error {
	out := cmd.OutOrStdout()
	reader := bufio.NewReader(cmd.InOrStdin())
	var failed int
	for _, r := range rep.Checks {
		_, _ = fmt.Fprintf(out, "[%s] %-20s %s\n", r.Status, r.Name, r.Summary)
		if r.Fix == nil || (r.Status != app.SeverityFail && r.Status != app.SeverityWarn) {
			continue
		}
		if !yes {
			_, _ = fmt.Fprintf(out, "Apply fix? (%s) [y/N]: ", r.Fix.Label)
			ans, _ := reader.ReadString('\n')
			ans = strings.ToLower(strings.TrimSpace(ans))
			if ans != "y" && ans != "yes" {
				_, _ = fmt.Fprintf(out, "[--] %s skipped\n", r.Name)
				continue
			}
		}
		// Fix.Command values are compile-time literals defined in
		// internal/app/doctor_checks.go, never external input. They MUST
		// stay literal-only for this shell invocation to remain safe.
		ec := exec.CommandContext(cmd.Context(), "sh", "-c", r.Fix.Command) //nolint:gosec // Fix.Command is a fixed internal literal, not user input
		ec.Stdout = out
		ec.Stderr = cmd.ErrOrStderr()
		if err := ec.Run(); err != nil {
			_, _ = fmt.Fprintf(out, "[FIX-FAILED] %s: %v\n", r.Name, err)
			failed++
			continue
		}
		_, _ = fmt.Fprintf(out, "[FIXED] %s\n", r.Name)
	}
	if failed > 0 {
		return fmt.Errorf("doctor: %d fix(es) failed", failed)
	}
	return nil
}
