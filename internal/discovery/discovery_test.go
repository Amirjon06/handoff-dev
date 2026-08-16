package discovery

import (
	"context"
	"net"
	"testing"

	"github.com/hashicorp/mdns"
)

func TestFromEntryBuildsDevice(t *testing.T) {
	device := FromEntry(&mdns.ServiceEntry{
		Name:       "amir-mac._staterelay._tcp.local.",
		Host:       "amir-mac.local.",
		AddrV4:     net.ParseIP("192.168.1.25"),
		Port:       8765,
		InfoFields: []string{"name=Amir Mac", "fingerprint=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "version=0.1.0-dev", "scheme=http"},
	})

	if device.Name != "Amir Mac" {
		t.Fatalf("name = %q", device.Name)
	}
	if device.Host != "amir-mac.local" {
		t.Fatalf("host = %q", device.Host)
	}
	if device.Endpoint != "http://192.168.1.25:8765" {
		t.Fatalf("endpoint = %q", device.Endpoint)
	}
	if device.Fingerprint == "" {
		t.Fatal("fingerprint is empty")
	}
}

func TestParseTXTIgnoresMalformedFields(t *testing.T) {
	fields := ParseTXT([]string{"name=desktop", "broken", "fingerprint=", " version = 0.1.0-dev "})

	if fields["name"] != "desktop" {
		t.Fatalf("name = %q", fields["name"])
	}
	if _, ok := fields["fingerprint"]; ok {
		t.Fatal("empty fingerprint was kept")
	}
	if fields["version"] != "0.1.0-dev" {
		t.Fatalf("version = %q", fields["version"])
	}
}

func TestAdvertiseValidatesOptions(t *testing.T) {
	err := Advertise(context.Background(), AdvertiseOptions{Port: 8765})
	if err == nil {
		t.Fatal("Advertise returned nil error without instance")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = Advertise(ctx, AdvertiseOptions{Instance: "test", Port: 0})
	if err == nil {
		t.Fatal("Advertise returned nil error with bad port")
	}
}
