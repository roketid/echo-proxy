package proxy

import (
	"fmt"
	"net"
	"strings"

	"github.com/labstack/echo/v4"
)

// TrustedProxiesEnv names the environment variable listing the proxies (IP
// addresses or CIDR ranges, comma-separated) whose X-Forwarded-For header is
// believed, e.g. Cloudflare's ranges or a load balancer's subnet.
const TrustedProxiesEnv = "TRUSTED_PROXIES"

// ParseTrustedProxies parses a comma-separated list of IP addresses and CIDR
// ranges. A bare address is taken as a single-host range. Blank entries are
// ignored, so an empty string returns no ranges.
func ParseTrustedProxies(list string) ([]*net.IPNet, error) {
	var ranges []*net.IPNet
	for _, entry := range strings.Split(list, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if !strings.Contains(entry, "/") {
			ip := net.ParseIP(entry)
			if ip == nil {
				return nil, fmt.Errorf("%q is not an IP address or CIDR range", entry)
			}
			bits := 128
			if ip.To4() != nil {
				ip, bits = ip.To4(), 32
			}
			ranges = append(ranges, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, ipNet, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("%q is not an IP address or CIDR range", entry)
		}
		ranges = append(ranges, ipNet)
	}
	return ranges, nil
}

// ClientIPExtractor decides where the client's IP comes from.
//
// With no trusted proxies it is the connection's own address: headers a client
// sends (X-Forwarded-For, X-Real-IP) cannot change it. With trusted proxies,
// X-Forwarded-For is read only when the connection comes from one of them,
// walking back past every trusted hop to the first address that is not. Only
// the listed ranges are trusted; echo's default trust of loopback, link-local
// and private addresses is turned off, so list those too if your proxy is on one.
func ClientIPExtractor(trusted []*net.IPNet) echo.IPExtractor {
	if len(trusted) == 0 {
		return echo.ExtractIPDirect()
	}
	options := []echo.TrustOption{
		echo.TrustLoopback(false),
		echo.TrustLinkLocal(false),
		echo.TrustPrivateNet(false),
	}
	for _, ipRange := range trusted {
		options = append(options, echo.TrustIPRange(ipRange))
	}
	return echo.ExtractIPFromXFFHeader(options...)
}
