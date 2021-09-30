package microgateway

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
)

// NetworkAddress contains the individual components
// for a parsed network address of the form accepted
// by ParseNetworkAddress(). Network should be a
// network value accepted by Go's net package. Port
// ranges are given by [StartPort, EndPort].
type NetworkAddress struct {
	Network   string
	Host      string
	StartPort uint
	EndPort   uint
}

// IsUnixNetwork returns true if na.Network is
// unix, unixgram, or unixpacket.
func (na NetworkAddress) IsUnixNetwork() bool {
	return isUnixNetwork(na.Network)
}

// JoinHostPort is like net.JoinHostPort, but where the port
// is StartPort + offset.
func (na NetworkAddress) JoinHostPort(offset uint) string {
	if na.IsUnixNetwork() {
		return na.Host
	}
	return net.JoinHostPort(na.Host, strconv.Itoa(int(na.StartPort+offset)))
}

// PortRangeSize returns how many ports are in
// pa's port range. Port ranges are inclusive,
// so the size is the difference of start and
// end ports plus one.
func (na NetworkAddress) PortRangeSize() uint {
	return (na.EndPort - na.StartPort) + 1
}

func (na NetworkAddress) isLoopback() bool {
	if na.IsUnixNetwork() {
		return true
	}
	if na.Host == "localhost" {
		return true
	}
	if ip := net.ParseIP(na.Host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func (na NetworkAddress) isWildcardInterface() bool {
	if na.Host == "" {
		return true
	}
	if ip := net.ParseIP(na.Host); ip != nil {
		return ip.IsUnspecified()
	}
	return false
}

func (na NetworkAddress) port() string {
	if na.StartPort == na.EndPort {
		return strconv.FormatUint(uint64(na.StartPort), 10)
	}
	return fmt.Sprintf("%d-%d", na.StartPort, na.EndPort)
}

// String reconstructs the address string to the form expected
// by ParseNetworkAddress(). If the address is a unix socket,
// any non-zero port will be dropped.
func (na NetworkAddress) String() string {
	return JoinNetworkAddress(na.Network, na.Host, na.port())
}

func isUnixNetwork(netw string) bool {
	return netw == "unix" || netw == "unixgram" || netw == "unixpacket"
}

// SplitNetworkAddress splits a into its network, host, and port components.
// Note that port may be a port range (:X-Y), or omitted for unix sockets.
func SplitNetworkAddress(a string) (network, host, port string, err error) {
	if idx := strings.Index(a, "/"); idx >= 0 {
		network = strings.ToLower(strings.TrimSpace(a[:idx]))
		a = a[idx+1:]
	}
	if isUnixNetwork(network) {
		host = a
		return
	}
	if !strings.Contains(a, ":") {
		a = ":" + a
	}
	host, port, err = net.SplitHostPort(a)
	return
}

// JoinNetworkAddress combines network, host, and port into a single
// address string of the form accepted by ParseNetworkAddress(). For
// unix sockets, the network should be "unix" (or "unixgram" or
// "unixpacket") and the path to the socket should be given as the
// host parameter.
func JoinNetworkAddress(network, host, port string) string {
	var a string
	if network != "" {
		a = network + "/"
	}
	if (host != "" && port == "") || isUnixNetwork(network) {
		a += host
	} else if port != "" {
		a += net.JoinHostPort(host, port)
	}
	return a
}

// ParseNetworkAddress parses addr into its individual
// components. The input string is expected to be of
// the form "network/host:port-range" where any part is
// optional. The default network, if unspecified, is tcp.
// Port ranges are inclusive.
//
// Network addresses are distinct from URLs and do not
// use URL syntax.
func ParseNetworkAddress(addr string) (NetworkAddress, error) {
	var host, port string
	network, host, port, err := SplitNetworkAddress(addr)
	if network == "" {
		network = "tcp"
	}
	if err != nil {
		return NetworkAddress{}, err
	}
	if isUnixNetwork(network) {
		return NetworkAddress{
			Network: network,
			Host:    host,
		}, nil
	}
	ports := strings.SplitN(port, "-", 2)
	if len(ports) == 1 {
		ports = append(ports, ports[0])
	}
	var start, end uint64
	start, err = strconv.ParseUint(ports[0], 10, 16)
	if err != nil {
		return NetworkAddress{}, fmt.Errorf("invalid start port: %v", err)
	}
	end, err = strconv.ParseUint(ports[1], 10, 16)
	if err != nil {
		return NetworkAddress{}, fmt.Errorf("invalid end port: %v", err)
	}
	if end < start {
		return NetworkAddress{}, fmt.Errorf("end port must not be less than start port")
	}
	if (end - start) > maxPortSpan {
		return NetworkAddress{}, fmt.Errorf("port range exceeds %d ports", maxPortSpan)
	}
	return NetworkAddress{
		Network:   network,
		Host:      host,
		StartPort: uint(start),
		EndPort:   uint(end),
	}, nil
}

func Listen(addr interface{}) (net.Listener, error) {
	l := ""
	switch a := addr.(type) {
	case int:
		l = strconv.Itoa(a)
	case string:
		l = a
	}
	lnAddr, err := ParseNetworkAddress(l)
	if err != nil {
		return nil, err
	}
	log.Printf("Listen: %v\n", l)
	for i := uint(0); i < lnAddr.PortRangeSize(); i++ {
		hostport := lnAddr.JoinHostPort(i)
		return net.Listen(lnAddr.Network, hostport)
	}
	return nil, fmt.Errorf("listen %v failed", addr)
}

const maxPortSpan = 65535
