package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// osvEndpoint is the Google OSV API endpoint.
var osvEndpoint = "https://api.osv.dev/v1/query"

func init() {
	if ep := os.Getenv("OSV_ENDPOINT"); ep != "" {
		osvEndpoint = ep
	}
}

const osvTimeout = 10 * time.Second

// osvQuery is the request payload for the OSV API.
type osvQuery struct {
	Package osvPackage `json:"package"`
	Version string     `json:"version,omitempty"`
}

type osvPackage struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
}

// osvResponse is the response from the OSV API.
type osvResponse struct {
	Vulns []osvVuln `json:"vulns"`
}

type osvVuln struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
}

// CheckPackageForMalware checks if an MCP server package has known malware.
// Returns an error message if malware is found, empty string if clean.
// Fail-open: network errors return empty string (allow).
func CheckPackageForMalware(command string, args []string) string {
	ecosystem := inferEcosystem(command)
	if ecosystem == "" {
		return ""
	}

	pkg, version := parsePackageFromArgs(args, ecosystem)
	if pkg == "" {
		return ""
	}

	malware, err := queryOSV(pkg, ecosystem, version)
	if err != nil {
		// Fail-open: network errors allow the package.
		return ""
	}

	if len(malware) == 0 {
		return ""
	}

	var ids []string
	for _, m := range malware {
		if len(ids) >= 3 {
			break
		}
		ids = append(ids, m.ID)
	}

	return fmt.Sprintf(
		"blocked: package '%s' (%s) has known malware advisories: %s",
		pkg, ecosystem, strings.Join(ids, ", "))
}

// inferEcosystem guesses the package ecosystem from the command name.
func inferEcosystem(command string) string {
	base := strings.ToLower(filepath.Base(command))
	switch base {
	case "npx", "npx.cmd":
		return "npm"
	case "uvx", "uvx.cmd", "pipx":
		return "PyPI"
	default:
		return ""
	}
}

// parsePackageFromArgs extracts package name and version from args.
func parsePackageFromArgs(args []string, ecosystem string) (string, string) {
	if len(args) == 0 {
		return "", ""
	}

	// Skip flags to find the package token.
	var token string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		token = arg
		break
	}
	if token == "" {
		return "", ""
	}

	switch ecosystem {
	case "npm":
		return parseNPMPackage(token)
	case "PyPI":
		return parsePyPIPackage(token)
	default:
		return token, ""
	}
}

// parseNPMPackage parses @scope/name@version or name@version.
func parseNPMPackage(token string) (string, string) {
	if strings.HasPrefix(token, "@") {
		// Scoped: @scope/name@version
		idx := strings.LastIndex(token[1:], "@")
		if idx < 0 {
			return token, ""
		}
		idx++ // adjust for the skip
		name := token[:idx]
		version := token[idx+1:]
		if version == "latest" {
			return name, ""
		}
		return name, version
	}

	// Unscoped: name@version
	if idx := strings.LastIndex(token, "@"); idx > 0 {
		name := token[:idx]
		version := token[idx+1:]
		if version == "latest" {
			return name, ""
		}
		return name, version
	}
	return token, ""
}

// parsePyPIPackage parses name[extras]==version.
func parsePyPIPackage(token string) (string, string) {
	// Strip extras: name[extra1,extra2]==version
	name := token
	if idx := strings.Index(name, "["); idx >= 0 {
		end := strings.Index(name, "]")
		if end > idx {
			name = name[:idx] + name[end+1:]
		}
	}
	if idx := strings.Index(name, "=="); idx >= 0 {
		return name[:idx], name[idx+2:]
	}
	return name, ""
}

// queryOSV queries the OSV API for MAL-* malware advisories.
func queryOSV(pkg, ecosystem, version string) ([]osvVuln, error) {
	query := osvQuery{
		Package: osvPackage{Name: pkg, Ecosystem: ecosystem},
		Version: version,
	}

	payload, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	client := &http.Client{Timeout: osvTimeout}
	req, err := http.NewRequest("POST", osvEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "hermesx-osv-check/2.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("osv api request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB max
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result osvResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	// Only malware advisories — ignore regular CVEs.
	var malware []osvVuln
	for _, v := range result.Vulns {
		if strings.HasPrefix(v.ID, "MAL-") {
			malware = append(malware, v)
		}
	}
	return malware, nil
}
