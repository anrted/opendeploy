package firewall

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/anrted/opendeploy/pkg/contract"
)

var (
	servicePortPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	commentPattern     = regexp.MustCompile(`^[\pL\pN _.,:/@+()-]{0,128}$`)
)

func validateRule(req *contract.FirewallRuleRequest) error {
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	req.Protocol = strings.ToLower(strings.TrimSpace(req.Protocol))
	req.Direction = strings.ToLower(strings.TrimSpace(req.Direction))
	req.IPVersion = strings.ToLower(strings.TrimSpace(req.IPVersion))
	req.Port = strings.TrimSpace(req.Port)
	req.Source = strings.TrimSpace(req.Source)
	req.Destination = strings.TrimSpace(req.Destination)
	req.Comment = strings.TrimSpace(req.Comment)

	if req.Action == "" {
		req.Action = "allow"
	}
	if req.Protocol == "" {
		req.Protocol = "any"
	}
	if req.Direction == "" {
		req.Direction = "in"
	}
	if req.IPVersion == "" {
		req.IPVersion = "both"
	}
	if !oneOf(req.Action, "allow", "deny", "reject") {
		return apperrors.InvalidInput("action must be allow, deny, or reject")
	}
	if !oneOf(req.Protocol, "tcp", "udp", "any") {
		return apperrors.InvalidInput("protocol must be tcp, udp, or any")
	}
	if !oneOf(req.Direction, "in", "out") {
		return apperrors.InvalidInput("direction must be in or out")
	}
	if !oneOf(req.IPVersion, "ipv4", "ipv6", "both") {
		return apperrors.InvalidInput("ip_version must be ipv4, ipv6, or both")
	}
	if req.Port == "" && req.Source == "" && req.Destination == "" {
		return apperrors.InvalidInput("port, source, or destination is required")
	}
	if req.Port != "" {
		if err := validatePort(req.Port); err != nil {
			return apperrors.InvalidInput(err.Error())
		}
	}
	for field, value := range map[string]string{"source": req.Source, "destination": req.Destination} {
		if err := validateAddress(field, value, req.IPVersion); err != nil {
			return err
		}
	}
	if !commentPattern.MatchString(req.Comment) {
		return apperrors.InvalidInput("comment contains unsupported characters or exceeds 128 characters")
	}
	return nil
}

func validateAddress(field, value, ipVersion string) error {
	if value == "" {
		return nil
	}
	if net.ParseIP(value) == nil {
		if _, _, err := net.ParseCIDR(value); err != nil {
			return apperrors.InvalidInput(field + " must be an IPv4/IPv6 address or CIDR")
		}
	}
	if ipVersion == "both" {
		return nil
	}
	ip := net.ParseIP(strings.Split(value, "/")[0])
	isV4 := ip != nil && ip.To4() != nil
	if (ipVersion == "ipv4" && !isV4) || (ipVersion == "ipv6" && isV4) {
		return apperrors.InvalidInput(field + " does not match ip_version")
	}
	return nil
}

func validatePort(value string) error {
	if servicePortPattern.MatchString(value) {
		return nil
	}
	separator := ":"
	if strings.Contains(value, "-") {
		separator = "-"
	}
	parts := strings.Split(value, separator)
	if len(parts) > 2 {
		return fmt.Errorf("port must be a number or a start:end range")
	}
	numbers := make([]int, len(parts))
	for index, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil || number < 1 || number > 65535 {
			return fmt.Errorf("port values must be between 1 and 65535")
		}
		numbers[index] = number
	}
	if len(numbers) == 2 && numbers[0] >= numbers[1] {
		return fmt.Errorf("port range start must be lower than its end")
	}
	return nil
}

func sameRule(left *contract.FirewallRule, right *contract.FirewallRuleRequest) bool {
	leftSource := strings.TrimSpace(left.Source)
	leftDestination := strings.TrimSpace(left.Destination)
	if strings.HasPrefix(strings.ToLower(leftSource), "anywhere") {
		leftSource = ""
	}
	if strings.HasPrefix(strings.ToLower(leftDestination), "anywhere") {
		leftDestination = ""
	}
	return left.Port == right.Port && left.Protocol == right.Protocol &&
		left.Action == right.Action && left.Direction == right.Direction &&
		leftSource == right.Source && leftDestination == right.Destination &&
		left.IPVersion == right.IPVersion
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
