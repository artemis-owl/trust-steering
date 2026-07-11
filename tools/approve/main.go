// approve is the human side of the steering approval gate: it lists pending
// work orders and grants or denies them. The gateway only ever treats an
// approval as valid when it was written here, out-of-band from the agent.
//
// Usage:
//
//	approve list
//	approve grant <correlation_id> [-approver <name>] [-ttl <duration>]
//	approve deny  <correlation_id> [-approver <name>]
//
// The store directory comes from APPROVALS_DIR (default: out/approvals).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Approval struct {
	CorrelationID string `json:"correlation_id"`
	Action        string `json:"action"`
	Resource      string `json:"resource"`
	Environment   string `json:"environment"`
	Status        string `json:"status"`
	RequestedAt   string `json:"requested_at"`
	Approver      string `json:"approver,omitempty"`
	ApprovedAt    string `json:"approved_at,omitempty"`
	ExpiresAt     string `json:"expires_at,omitempty"`
	ConsumedAt    string `json:"consumed_at,omitempty"`
}

var correlationIDPattern = regexp.MustCompile(`^corr-[0-9a-f]{16}$`)

func approvalsDir() string {
	if d := os.Getenv("APPROVALS_DIR"); d != "" {
		return d
	}
	if root := os.Getenv("STEERING_ROOT"); root != "" {
		return filepath.Join(root, "out", "approvals")
	}
	return "out/approvals"
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "list":
		if err := list(); err != nil {
			fatal(err)
		}
	case "grant":
		if err := resolve(os.Args[2:], "approved"); err != nil {
			fatal(err)
		}
	case "deny":
		if err := resolve(os.Args[2:], "denied"); err != nil {
			fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: approve list | approve grant <correlation_id> [-approver <name>] [-ttl <duration>] | approve deny <correlation_id> [-approver <name>]")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "approve:", err)
	os.Exit(1)
}

func list() error {
	entries, err := filepath.Glob(filepath.Join(approvalsDir(), "*.json*"))
	if err != nil {
		return err
	}
	var approvals []Approval
	for _, path := range entries {
		if strings.HasSuffix(path, ".tmp") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var a Approval
		if err := json.Unmarshal(data, &a); err != nil {
			continue
		}
		approvals = append(approvals, a)
	}
	if len(approvals) == 0 {
		fmt.Printf("no approval records in %s\n", approvalsDir())
		return nil
	}
	sort.Slice(approvals, func(i, j int) bool { return approvals[i].RequestedAt < approvals[j].RequestedAt })
	fmt.Printf("%-22s %-8s %-20s %-8s %-9s %-12s\n", "CORRELATION_ID", "ACTION", "RESOURCE", "ENV", "STATUS", "APPROVER")
	for _, a := range approvals {
		fmt.Printf("%-22s %-8s %-20s %-8s %-9s %-12s\n", a.CorrelationID, a.Action, a.Resource, a.Environment, a.Status, a.Approver)
	}
	return nil
}

func resolve(args []string, status string) error {
	if len(args) < 1 || strings.HasPrefix(args[0], "-") {
		usage()
		return fmt.Errorf("missing correlation_id")
	}
	correlationID := args[0]
	if !correlationIDPattern.MatchString(correlationID) {
		return fmt.Errorf("invalid correlation_id %q", correlationID)
	}

	fs := flag.NewFlagSet(status, flag.ExitOnError)
	approver := fs.String("approver", defaultApprover(), "identity recorded as the approver")
	ttl := fs.Duration("ttl", 15*time.Minute, "how long a grant stays valid (grant only)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *approver == "" {
		return fmt.Errorf("approver identity required (pass -approver or set $USER)")
	}

	path := filepath.Join(approvalsDir(), correlationID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("no approval record for %s (is the work order pending?): %w", correlationID, err)
	}
	var a Approval
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	// Pending records resolve either way; an approved-but-never-consumed grant
	// (e.g. expired before use) may be re-granted to refresh its TTL. Denied
	// and consumed records are immutable.
	regrant := a.Status == "approved" && status == "approved"
	if a.Status != "pending" && !regrant {
		return fmt.Errorf("record %s is %q and cannot be resolved to %q; request a new work order", correlationID, a.Status, status)
	}

	now := time.Now().UTC()
	a.Status = status
	a.Approver = *approver
	a.ApprovedAt = now.Format(time.RFC3339)
	if status == "approved" {
		a.ExpiresAt = now.Add(*ttl).Format(time.RFC3339)
	}

	out, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	if status == "approved" {
		fmt.Printf("granted %s (%s %s in %s) as %s, valid until %s\n", correlationID, a.Action, a.Resource, a.Environment, a.Approver, a.ExpiresAt)
	} else {
		fmt.Printf("denied %s as %s\n", correlationID, a.Approver)
	}
	return nil
}

func defaultApprover() string {
	if u := os.Getenv("USER"); u != "" {
		return "user:" + u
	}
	return ""
}
