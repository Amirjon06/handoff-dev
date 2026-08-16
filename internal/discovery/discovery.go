package discovery

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
)

const ServiceName = "_staterelay._tcp"

type Device struct {
	Name        string
	Host        string
	Address     string
	Port        int
	Fingerprint string
	Version     string
	Endpoint    string
}

type AdvertiseOptions struct {
	Instance    string
	Name        string
	Fingerprint string
	Version     string
	Scheme      string
	Port        int
}

func Advertise(ctx context.Context, options AdvertiseOptions) error {
	if strings.TrimSpace(options.Instance) == "" {
		return fmt.Errorf("advertise instance is required")
	}
	if options.Port <= 0 || options.Port > 65535 {
		return fmt.Errorf("advertise port must be between 1 and 65535")
	}

	service, err := mdns.NewMDNSService(options.Instance, ServiceName, "", "", options.Port, nil, txtFields(options))
	if err != nil {
		return err
	}
	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return err
	}
	defer server.Shutdown()

	<-ctx.Done()
	if err := ctx.Err(); err != nil && err != context.Canceled {
		return err
	}
	return nil
}

func Lookup(ctx context.Context, timeout time.Duration) ([]Device, error) {
	if timeout <= 0 {
		timeout = time.Second
	}
	entries := make(chan *mdns.ServiceEntry, 16)
	params := mdns.DefaultParams(ServiceName)
	params.Timeout = timeout
	params.Entries = entries

	if err := mdns.QueryContext(ctx, params); err != nil {
		return nil, err
	}
	close(entries)

	devices := make([]Device, 0)
	seen := make(map[string]bool)
	for entry := range entries {
		device := FromEntry(entry)
		key := device.Fingerprint
		if key == "" {
			key = device.Endpoint
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		devices = append(devices, device)
	}

	sort.Slice(devices, func(i int, j int) bool {
		if devices[i].Name == devices[j].Name {
			return devices[i].Endpoint < devices[j].Endpoint
		}
		return devices[i].Name < devices[j].Name
	})
	return devices, nil
}

func FromEntry(entry *mdns.ServiceEntry) Device {
	fields := ParseTXT(entry.InfoFields)
	address := entryAddress(entry)
	device := Device{
		Name:        firstNonEmpty(fields["name"], strings.TrimSuffix(entry.Name, ".")),
		Host:        strings.TrimSuffix(entry.Host, "."),
		Address:     address,
		Port:        entry.Port,
		Fingerprint: fields["fingerprint"],
		Version:     fields["version"],
	}
	if address != "" && entry.Port > 0 {
		device.Endpoint = (&url.URL{
			Scheme: firstNonEmpty(fields["scheme"], "http"),
			Host:   net.JoinHostPort(address, strconv.Itoa(entry.Port)),
		}).String()
	}
	return device
}

func ParseTXT(fields []string) map[string]string {
	out := make(map[string]string, len(fields))
	for _, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			out[key] = value
		}
	}
	return out
}

func txtFields(options AdvertiseOptions) []string {
	fields := []string{"scheme=" + firstNonEmpty(options.Scheme, "http")}
	if strings.TrimSpace(options.Name) != "" {
		fields = append(fields, "name="+strings.TrimSpace(options.Name))
	}
	if strings.TrimSpace(options.Fingerprint) != "" {
		fields = append(fields, "fingerprint="+strings.ToLower(strings.TrimSpace(options.Fingerprint)))
	}
	if strings.TrimSpace(options.Version) != "" {
		fields = append(fields, "version="+strings.TrimSpace(options.Version))
	}
	sort.Strings(fields)
	return fields
}

func entryAddress(entry *mdns.ServiceEntry) string {
	if entry.AddrV4 != nil {
		return entry.AddrV4.String()
	}
	if entry.AddrV6IPAddr != nil {
		return entry.AddrV6IPAddr.IP.String()
	}
	if entry.AddrV6 != nil {
		return entry.AddrV6.String()
	}
	if entry.Addr != nil {
		return entry.Addr.String()
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
