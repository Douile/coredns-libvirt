package virt

import (
	"sync"
	"time"
	"strings"
	"fmt"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"

	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
)

var log = clog.NewWithPlugin("virt")

func init() { plugin.Register("virt", setup) }

func setup(c *caddy.Controller) error {
	c.Next()

	vm := new(VirtMachine)
	vm.ConnectMutex = new(sync.Mutex)

	if c.NextArg() {
		vm.TLD = c.Val()

		if !strings.HasSuffix(vm.TLD, ".") {
			vm.TLD = fmt.Sprintf("%s.", vm.TLD)
		}

		if !strings.HasPrefix(vm.TLD, ".") {
			vm.TLD = fmt.Sprintf(".%s", vm.TLD)
		}
	} else {
		vm.TLD = ".virt."
	}

	socketPath := "/run/libvirt/libvirt-sock-ro"
	if c.NextArg() {
		socketPath = c.Val()
	}

	connectUri := libvirt.QEMUSystem
	if c.NextArg() {
		connectUri = libvirt.ConnectURI(c.Val())
	}
	vm.ConnectURI = connectUri

	vm.ShouldDisconnect = c.NextArg() && c.Val() == "yes"

	if c.NextArg() {
		return plugin.Error("virt", c.ArgErr())
	}

	if len(socketPath) > 0 {
		vm.LibVirt = libvirt.NewWithDialer(dialers.NewLocal(dialers.WithLocalTimeout(5*time.Second), dialers.WithSocket(socketPath)))
	} else {
		vm.LibVirt = libvirt.NewWithDialer(dialers.NewLocal(dialers.WithLocalTimeout(5 * time.Second)))
	}

	// Add the Plugin to CoreDNS, so Servers can use it in their plugin chain.
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		vm.Next = next

		return vm
	})

	return nil
}
