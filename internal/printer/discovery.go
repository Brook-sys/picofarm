package printer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

// DiscoveredPrinter represents a printer found on the network.
type DiscoveredPrinter struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Host         string               `json:"host"`
	Port         int                  `json:"port"`
	Type         model.ConnectionType `json:"type"`
	Model        string               `json:"model,omitempty"`
	Manufacturer string               `json:"manufacturer,omitempty"`
	Version      string               `json:"version,omitempty"`
	SerialNumber string               `json:"serial_number,omitempty"`
	AlreadyAdded bool                 `json:"already_added"`
}

// Discovery handles automatic printer discovery on the local network.
type Discovery struct {
	timeout time.Duration
}

// NewDiscovery creates a new printer discovery instance.
func NewDiscovery() *Discovery {
	return &Discovery{
		timeout: 3 * time.Second,
	}
}

// scanDeadline bounds one full network scan. Each probe gets d.timeout, and the
// batches are sequential, so a /24 with 254 hosts is ~5 minutes in the worst
// case (254 × 3s ÷ 20 concurrent). Past that the caller is better served by a
// partial result than by a request that never completes.
const scanDeadline = 5 * time.Minute

// ScanNetwork discovers printers on the local network.
// Uses sequential scanning to avoid file descriptor exhaustion.
func (d *Discovery) ScanNetwork(ctx context.Context) ([]DiscoveredPrinter, error) {
	var printers []DiscoveredPrinter

	// A full scan is 254+ probes; without an overall deadline a caller that
	// passes a context without one (or one that is never cancelled) leaves the
	// goroutines probing forever.
	scanCtx, cancel := context.WithTimeout(ctx, scanDeadline)
	defer cancel()

	// Get local network range
	localIP, network, err := getLocalNetwork()
	if err != nil {
		return nil, fmt.Errorf("failed to get local network: %w", err)
	}
	slog.Info("scanning network", "local_ip", localIP, "network", network)

	// Generate IP range to scan
	ips := generateIPRange(network)
	slog.Info("scanning IPs", "count", len(ips))

	// Scan sequentially with batches to avoid FD exhaustion
	batchSize := 20
	for i := 0; i < len(ips); i += batchSize {
		// The per-probe HTTP calls honor the context, but nothing stopped the
		// outer loop: a client that disconnected kept the whole scan running.
		if err := scanCtx.Err(); err != nil {
			slog.Info("network scan stopped early", "scanned", i, "of", len(ips), "reason", err)
			return printers, nil
		}

		end := i + batchSize
		if end > len(ips) {
			end = len(ips)
		}
		batch := ips[i:end]

		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, ip := range batch {
			wg.Add(1)
			go func(ip string) {
				defer wg.Done()
				found := d.probeHost(ctx, ip)
				if len(found) > 0 {
					mu.Lock()
					printers = append(printers, found...)
					mu.Unlock()
				}
			}(ip)
		}
		wg.Wait()

		// Small delay between batches
		time.Sleep(50 * time.Millisecond)
	}

	slog.Info("discovery complete", "found", len(printers))
	return printers, nil
}

// probeHost checks a single host for known printer services.
func (d *Discovery) probeHost(ctx context.Context, host string) []DiscoveredPrinter {
	var found []DiscoveredPrinter

	// Check common printer ports
	ports := []struct {
		port  int
		check func(ctx context.Context, host string, port int) *DiscoveredPrinter
	}{
		{80, d.checkOctoPrint},   // OctoPrint default
		{5000, d.checkOctoPrint}, // OctoPrint alt port
		{7125, d.checkMoonraker}, // Moonraker default
		{80, d.checkMoonraker},   // Moonraker behind proxy
		{8883, d.checkBambu},     // Bambu Lab MQTT
		{3000, d.checkChiTu},     // ChiTu/Elegoo resin printers
		{6000, d.checkChiTu},     // ChiTu alt port
	}

	for _, p := range ports {
		// Quick TCP check first
		if !d.isPortOpen(host, p.port) {
			continue
		}

		// Detailed service check
		if printer := p.check(ctx, host, p.port); printer != nil {
			// Avoid duplicates
			isDupe := false
			for _, existing := range found {
				if existing.Host == printer.Host && existing.Type == printer.Type {
					isDupe = true
					break
				}
			}
			if !isDupe {
				found = append(found, *printer)
			}
		}
	}

	return found
}

// isPortOpen does a quick TCP port check.
func (d *Discovery) isPortOpen(host string, port int) bool {
	return d.isPortOpenTimeout(host, port, 500*time.Millisecond)
}

// isPortOpenTimeout does a TCP port check with a custom timeout.
func (d *Discovery) isPortOpenTimeout(host string, port int, timeout time.Duration) bool {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false
	}
	// Ensure connection is fully closed
	conn.SetDeadline(time.Now())
	conn.Close()
	return true
}

// checkOctoPrint probes for OctoPrint API.
func (d *Discovery) checkOctoPrint(ctx context.Context, host string, port int) *DiscoveredPrinter {
	url := fmt.Sprintf("http://%s:%d/api/version", host, port)

	// Use a transport that closes idle connections
	transport := &http.Transport{
		DisableKeepAlives: true,
	}
	client := &http.Client{Timeout: d.timeout, Transport: transport}
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	// OctoPrint returns 200 even without API key for version endpoint
	if resp.StatusCode != 200 {
		return nil
	}

	// Must parse the response and verify it's actually OctoPrint
	var version struct {
		API    string `json:"api"`
		Server string `json:"server"`
		Text   string `json:"text"`
	}

	body := make([]byte, 1024)
	n, _ := resp.Body.Read(body)
	bodyStr := string(body[:n])

	// Verify this is OctoPrint - must have "OctoPrint" in response
	if !strings.Contains(bodyStr, "OctoPrint") && !strings.Contains(bodyStr, "octoprint") {
		return nil
	}

	// Parse JSON for version info
	if err := json.Unmarshal(body[:n], &version); err != nil {
		// Still return if we found OctoPrint string
		version.Server = "unknown"
	}

	name := fmt.Sprintf("OctoPrint @ %s", host)

	return &DiscoveredPrinter{
		ID:           uuid.New().String(),
		Name:         name,
		Host:         host,
		Port:         port,
		Type:         model.ConnectionTypeOctoPrint,
		Version:      version.Server,
		Manufacturer: "OctoPrint",
	}
}

// checkMoonraker probes for Moonraker API.
func (d *Discovery) checkMoonraker(ctx context.Context, host string, port int) *DiscoveredPrinter {
	url := fmt.Sprintf("http://%s:%d/server/info", host, port)

	transport := &http.Transport{DisableKeepAlives: true}
	client := &http.Client{Timeout: d.timeout, Transport: transport}
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil
	}

	// Must verify it's actually Moonraker
	body := make([]byte, 2048)
	n, _ := resp.Body.Read(body)
	bodyStr := string(body[:n])

	// Moonraker response should contain "moonraker" or "klippy"
	isMoonraker := strings.Contains(strings.ToLower(bodyStr), "moonraker") ||
		strings.Contains(strings.ToLower(bodyStr), "klippy") ||
		strings.Contains(strings.ToLower(bodyStr), "klipper")

	if !isMoonraker {
		return nil
	}

	name := fmt.Sprintf("Klipper @ %s", host)

	return &DiscoveredPrinter{
		ID:           uuid.New().String(),
		Name:         name,
		Host:         host,
		Port:         port,
		Type:         model.ConnectionTypeMoonraker,
		Manufacturer: "Klipper/Moonraker",
	}
}

// checkChiTu probes for ChiTu-based resin printers (Elegoo, Anycubic, Phrozen, etc.).
// These printers run a web server typically on port 3000 or 6000.
func (d *Discovery) checkChiTu(ctx context.Context, host string, port int) *DiscoveredPrinter {
	// ChiTu printers have endpoints like /getSysInfo or /getSystemInfo
	endpoints := []string{
		fmt.Sprintf("http://%s:%d/getSysInfo", host, port),
		fmt.Sprintf("http://%s:%d/getSystemInfo", host, port),
		fmt.Sprintf("http://%s:%d/system/info", host, port),
	}

	transport := &http.Transport{DisableKeepAlives: true}
	client := &http.Client{Timeout: d.timeout, Transport: transport}

	for _, url := range endpoints {
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			continue
		}

		body := make([]byte, 2048)
		n, _ := resp.Body.Read(body)
		resp.Body.Close()

		bodyStr := string(body[:n])
		bodyLower := strings.ToLower(bodyStr)

		// Check for ChiTu/resin printer indicators
		isChiTu := strings.Contains(bodyLower, "chitu") ||
			strings.Contains(bodyLower, "elegoo") ||
			strings.Contains(bodyLower, "anycubic") ||
			strings.Contains(bodyLower, "phrozen") ||
			strings.Contains(bodyLower, "creality") ||
			strings.Contains(bodyLower, "mars") ||
			strings.Contains(bodyLower, "saturn") ||
			strings.Contains(bodyLower, "resin") ||
			strings.Contains(bodyLower, "lcd") ||
			strings.Contains(bodyLower, "msla")

		if isChiTu {
			var name string
			manufacturer := "Unknown"

			// Try to identify manufacturer
			if strings.Contains(bodyLower, "elegoo") {
				manufacturer = "Elegoo"
				name = fmt.Sprintf("Elegoo @ %s", host)
			} else if strings.Contains(bodyLower, "anycubic") {
				manufacturer = "Anycubic"
				name = fmt.Sprintf("Anycubic @ %s", host)
			} else if strings.Contains(bodyLower, "phrozen") {
				manufacturer = "Phrozen"
				name = fmt.Sprintf("Phrozen @ %s", host)
			} else if strings.Contains(bodyLower, "creality") {
				manufacturer = "Creality"
				name = fmt.Sprintf("Creality @ %s", host)
			} else {
				name = fmt.Sprintf("Resin Printer @ %s", host)
			}

			slog.Info("found ChiTu resin printer", "host", host, "port", port, "manufacturer", manufacturer)

			return &DiscoveredPrinter{
				ID:           uuid.New().String(),
				Name:         name,
				Host:         host,
				Port:         port,
				Type:         model.ConnectionTypeChiTu,
				Manufacturer: manufacturer,
			}
		}
	}

	return nil
}

// bambuUDPInfo holds parsed fields from a Bambu UDP discovery response.
type bambuUDPInfo struct {
	DevName        string `json:"dev_name"`
	DevID          string `json:"dev_id"`
	DevProductName string `json:"dev_product_name"`
}

// probeBambuUDP sends M99999 to host:2021 and parses the JSON response
// to get the printer's real name, serial number and model.
// Tries both unicast (direct) and broadcast approaches.
func (d *Discovery) probeBambuUDP(host string) *bambuUDPInfo {
	// Try unicast first (direct to the printer)
	if info := d.probeBambuUDPUnicast(host); info != nil {
		return info
	}

	// Try broadcast — some Bambu printers only respond to broadcast on port 2021
	return d.probeBambuUDPBroadcast(host)
}

func (d *Discovery) probeBambuUDPUnicast(host string) *bambuUDPInfo {
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:2021", host))
	if err != nil {
		slog.Debug("Bambu UDP unicast resolve failed", "host", host, "error", err)
		return nil
	}

	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		slog.Debug("Bambu UDP unicast dial failed", "host", host, "error", err)
		return nil
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(3 * time.Second)) //nolint:errcheck // best-effort deadline

	if _, err := conn.Write([]byte("M99999")); err != nil {
		slog.Debug("Bambu UDP unicast write failed", "host", host, "error", err)
		return nil
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		slog.Debug("Bambu UDP unicast no response", "host", host, "error", err)
		return nil
	}

	return d.parseBambuUDPResponse(host, buf[:n])
}

func (d *Discovery) probeBambuUDPBroadcast(targetHost string) *bambuUDPInfo {
	broadcastAddr, err := net.ResolveUDPAddr("udp4", "255.255.255.255:2021")
	if err != nil {
		return nil
	}

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: 0})
	if err != nil {
		slog.Debug("Bambu UDP broadcast listen failed", "error", err)
		return nil
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(4 * time.Second)) //nolint:errcheck // best-effort deadline

	// Send broadcast discovery
	for i := 0; i < 2; i++ {
		conn.WriteToUDP([]byte("M99999"), broadcastAddr)
		time.Sleep(100 * time.Millisecond)
	}

	// Read responses and look for our target host
	buf := make([]byte, 4096)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		if remoteAddr.IP.String() == targetHost {
			if info := d.parseBambuUDPResponse(targetHost, buf[:n]); info != nil {
				return info
			}
		}
	}

	slog.Debug("Bambu UDP broadcast got no match", "target", targetHost)
	return nil
}

func (d *Discovery) parseBambuUDPResponse(host string, data []byte) *bambuUDPInfo {
	var info bambuUDPInfo
	if err := json.Unmarshal(data, &info); err != nil {
		slog.Debug("failed to parse Bambu UDP response", "host", host, "error", err, "raw", string(data))
		return nil
	}

	slog.Info("Bambu UDP probe success", "host", host, "name", info.DevName, "serial", info.DevID, "model", info.DevProductName)
	return &info
}

// checkBambu probes for Bambu Lab printers.
// Bambu printers use MQTT on port 8883 and have FTPS on port 990.
func (d *Discovery) checkBambu(ctx context.Context, host string, port int) *DiscoveredPrinter {
	// Use a longer timeout for Bambu-specific port checks since these
	// are secondary verification on an already-responsive host.
	bambuTimeout := 1500 * time.Millisecond

	// Check Bambu-specific ports with generous timeout
	hasMQTT := d.isPortOpenTimeout(host, 8883, bambuTimeout)
	if !hasMQTT {
		return nil
	}

	hasFTPS := d.isPortOpenTimeout(host, 990, bambuTimeout)

	isBambu := false

	// MQTT + FTPS is definitive Bambu identification
	if hasFTPS {
		slog.Info("found Bambu printer (MQTT+FTPS)", "host", host)
		isBambu = true
	}

	if !isBambu {
		// MQTT only - check port 21 for FTP banner as secondary confirmation
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, "21"), bambuTimeout)
		if err == nil {
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			banner := make([]byte, 512)
			n, _ := conn.Read(banner)
			conn.Close()
			bannerStr := strings.ToLower(string(banner[:n]))
			if strings.Contains(bannerStr, "bambu") || strings.Contains(bannerStr, "bbl") {
				slog.Info("found Bambu printer (MQTT+FTP banner)", "host", host)
				isBambu = true
			}
		}
	}

	if !isBambu {
		// MQTT-only on a local network: still likely Bambu.
		// Port 8883 (MQTTS) is uncommon outside IoT/printers on home networks.
		slog.Info("found probable Bambu printer (MQTT only)", "host", host)
	}

	printer := &DiscoveredPrinter{
		ID:           uuid.New().String(),
		Name:         fmt.Sprintf("Bambu Printer @ %s", host),
		Host:         host,
		Port:         8883,
		Type:         model.ConnectionTypeBambuLAN,
		Manufacturer: "Bambu Lab",
	}

	// Try UDP probe to get real name, model and serial number
	if info := d.probeBambuUDP(host); info != nil {
		if info.DevName != "" {
			printer.Name = info.DevName
		}
		if info.DevProductName != "" {
			printer.Model = info.DevProductName
		}
		if info.DevID != "" {
			printer.SerialNumber = info.DevID
		}
	}

	return printer
}

// ScanNetworkEnv pins the subnet to scan. In a container on a Docker bridge
// network the first non-loopback interface belongs to the container, not to the
// LAN where the printers live, so the operator needs a way to name the subnet
// explicitly (or to run the container with the host network).
const ScanNetworkEnv = "PICOFARM_SCAN_NETWORK"

// getLocalNetwork returns the local IP and network CIDR.
//
// The CIDR comes from PICOFARM_SCAN_NETWORK when set. Interface enumeration
// alone is not enough in a container: on a bridge network the first
// non-loopback IPv4 is the container's own address, so the scan would dutifully
// probe the container's subnet and find nothing.
func getLocalNetwork() (string, *net.IPNet, error) {
	if raw := strings.TrimSpace(os.Getenv(ScanNetworkEnv)); raw != "" {
		ip, network, err := net.ParseCIDR(raw)
		if err != nil {
			return "", nil, fmt.Errorf("%s is not a valid CIDR: %q", ScanNetworkEnv, raw)
		}
		return ip.String(), network, nil
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", nil, err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), ipnet, nil
			}
		}
	}

	return "", nil, fmt.Errorf("no suitable network interface found")
}

// maxScanHosts bounds a single scan. Honoring the mask on a /8 or /16 means
// hundreds of thousands to millions of probes, which is a self-inflicted denial
// of service on the host and on the printers being probed. Above this the scan
// is bounded and says so, instead of taking the whole server down with it.
const maxScanHosts = 1024

// generateIPRange returns the usable host addresses inside the network, honoring
// the real prefix length.
//
// The previous implementation always varied the last octet and stopped at 254
// regardless of the mask. On a /16 that meant scanning a single /24 out of 256
// and missing every printer outside it; on a /28 it probed ~192 addresses that
// are not on the local link at all; and on a /25 or /26 the hosts past the real
// boundary are on someone else's segment.
func generateIPRange(network *net.IPNet) []string {
	base := network.IP.Mask(network.Mask).To4()
	if base == nil {
		return nil
	}
	ones, bits := network.Mask.Size()
	if bits != 32 {
		// Not an IPv4 mask; nothing sensible to enumerate.
		return nil
	}

	// uint64 arithmetic: a /0 is 2^32 addresses and would overflow int32 math.
	start := uint64(base[0])<<24 | uint64(base[1])<<16 | uint64(base[2])<<8 | uint64(base[3])
	blockSize := uint64(1) << (32 - ones)
	broadcast := start + blockSize - 1
	// The first and last addresses of the block are unusable (network and
	// broadcast) on any prefix up to /30.
	first := start + 1
	last := broadcast - 1
	if last < first {
		// /31 and /32 have no usable host range.
		return nil
	}
	truncated := false
	if last-first+1 > maxScanHosts {
		last = first + maxScanHosts - 1
		truncated = true
	}

	ips := make([]string, 0, last-first+1)
	for ip := first; ip <= last; ip++ {
		ips = append(ips, net.IPv4(byte(ip>>24), byte(ip>>16), byte(ip>>8), byte(ip)).String())
	}
	if truncated {
		slog.Warn("network is larger than the scan cap, scanning only part of it",
			"network", network.String(), "usable_hosts", broadcast-first-1, "scanned", len(ips))
	}
	return ips
}

// QuickScan is the entry point the API uses for printer discovery.
//
// It is not a partial scan: it walks the whole subnet and probes the same ports
// as ScanNetwork, then deduplicates by host+type. The name and the older comments
// here ("only the most common ports", "UDP discovery disabled") describe a lighter
// variant that does not exist — UDP discovery was removed, so the TCP sweep of
// the entire range is all there is. A /24 therefore costs 254 probes × the port
// list, in batches of 20, which is why the scan has its own deadline and why the
// subnet can be pinned with PICOFARM_SCAN_NETWORK.
func (d *Discovery) QuickScan(ctx context.Context) ([]DiscoveredPrinter, error) {
	printers, err := d.ScanNetwork(ctx)
	if err != nil {
		return nil, err
	}

	// Deduplicate by host
	seen := make(map[string]bool)
	var unique []DiscoveredPrinter
	for _, p := range printers {
		key := fmt.Sprintf("%s:%s", p.Host, p.Type)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, p)
		}
	}

	return unique, nil
}
