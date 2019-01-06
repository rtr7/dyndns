package dyndns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vishvananda/netlink"
)

var numericRe = regexp.MustCompile(`^[0-9]+$`)

func running(base string) bool {
	fis, err := ioutil.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, fi := range fis {
		if !fi.IsDir() {
			continue
		}
		if !numericRe.MatchString(fi.Name()) {
			continue
		}
		cmdline, err := ioutil.ReadFile(filepath.Join("/proc", fi.Name(), "cmdline"))
		if err != nil {
			if os.IsNotExist(err) {
				continue // process vanished
			}
			return false
		}
		if idx := bytes.IndexByte(cmdline, '\x00'); idx > -1 {
			cmdline = cmdline[:idx]
		}
		if filepath.Base(string(cmdline)) == base {
			return true
		}
	}
	return false
}

type interfaceDetails struct {
	Name string `json:"name"` // e.g. uplink0, or lan0
	Addr string `json:"addr"` // e.g. 192.168.42.1/24
}

type interfaceConfig struct {
	Interfaces []interfaceDetails `json:"interfaces"`
}

// linkAddress returns the IP address configured for the interface ifname in
// interfaces.json.
func linkAddress(dir, ifname string) (net.IP, error) {
	fn := filepath.Join(dir, "interfaces.json")
	b, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}
	var cfg interfaceConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	for _, details := range cfg.Interfaces {
		if details.Name != ifname {
			continue
		}
		ip, _, err := net.ParseCIDR(details.Addr)
		return ip, err
	}
	return nil, fmt.Errorf("%s does not configure interface %q", fn, ifname)
}

func defaultAddr() (string, error) {
	var gw string
	if running("dnsd") {
		// This process is running on router7
		addr, err := linkAddress("/perm", "lan0")
		if err != nil {
			return "", err
		}
		gw = addr.String()
	} else {
		route, err := netlink.RouteGet(net.ParseIP("192.0.2.0" /* RFC5737 TEST-NET-1 */))
		if err != nil {
			return "", err
		}
		if got, want := len(route), 1; got != want {
			return "", fmt.Errorf("route get 0.0.0.0: got %v, want %v", got, want)
		}
		gw = route[0].Gw.String()
	}
	const dnsdPort = 8053
	return fmt.Sprintf("http://%s:%d/dyndns", gw, dnsdPort), nil
}

// SetSubname sets host.<hostname> to ip on this network’s router7.
func SetSubname(host string, ip net.IP) error {
	if ip == nil {
		return fmt.Errorf("BUG: ip == nil")
	}
	addr, err := defaultAddr()
	if err != nil {
		return err
	}
	resp, err := http.PostForm(addr, url.Values{
		"host": []string{host},
		"ip":   []string{ip.String()},
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if got, want := resp.StatusCode, 200; got != want {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("%s: unexpected HTTP status %v, want %v (%s)", addr, resp.Status, want, strings.TrimSpace(string(body)))
	}
	return nil
}
