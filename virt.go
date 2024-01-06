package virt

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"

	"github.com/digitalocean/go-libvirt"
	"github.com/miekg/dns"
)

type VirtMachine struct {
	Next             plugin.Handler
	TLD              string
	ConnectURI       libvirt.ConnectURI
	LibVirt          *libvirt.Libvirt
	ConnectMutex     *sync.Mutex
	ShouldDisconnect bool
}

// ServeDNS implements the plugin.Handler interface.
func (p VirtMachine) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	qname := state.Name()

	wrappedTLD := fmt.Sprintf(".%s.", p.TLD)

	if !strings.HasSuffix(qname, wrappedTLD) || (state.QType() != dns.TypeA && state.QType() != dns.TypeAAAA && state.QType() != dns.TypeTXT) {
		return plugin.NextOrFailure(p.Name(), p.Next, ctx, w, r)
	}

	domName := strings.TrimSuffix(qname, wrappedTLD)

	if p.ShouldDisconnect {
		// Locking is only needed if we disconnect after
		// every query. Otherwise implicit write locks are
		// enough.
		p.ConnectMutex.Lock()
		defer p.ConnectMutex.Unlock()
	}

	if !p.LibVirt.IsConnected() {
		log.Info("Connecting to libvirt...")
		err := p.LibVirt.ConnectToURI(p.ConnectURI)
		if err != nil {
			log.Warningf("Unable to dial libvirt: %v", err)
			//return plugin.NextOrFailure(p.Name(), p.Next, ctx, w, r)
		}
	}

	domPtr, err := p.LibVirt.DomainLookupByName(domName)
	if err != nil {
		log.Infof("Error Domain doesn't exist (%s): %v", domName, err)
		return plugin.NextOrFailure(p.Name(), p.Next, ctx, w, r)
	}

	//addrs, err := p.LibVirt.DomainInterfaceAddresses(domPtr, libvirt.DomainInterfaceAddressesSrcLease, 0)
	ifaces, err := p.LibVirt.DomainInterfaceAddresses(domPtr, 0, 0)
	if err != nil {
		log.Warningf("Error fetching interface addresses: %v", err)
		return plugin.NextOrFailure(p.Name(), p.Next, ctx, w, r)
	}

	// TODO: Add disconnect after n secs option
	if p.ShouldDisconnect {
		err = p.LibVirt.ConnectClose()
		if err != nil {
			log.Warningf("Unable to close libvirt connection: %v", err)
		}
	}

	answers := []dns.RR{}
	for _, iface := range ifaces {
		for _, addr := range iface.Addrs {
			log.Infof("Replying with address: %s", addr.Addr)
			ip := net.ParseIP(addr.Addr)
			if ip.To4() != nil && state.QType() == dns.TypeA {
				rr := new(dns.A)
				rr.Hdr = dns.RR_Header{Name: qname, Rrtype: dns.TypeA, Class: dns.ClassINET}
				rr.A = ip.To4()

				answers = append(answers, rr)
			} else if ip.To4() == nil && ip.To16() != nil && state.QType() == dns.TypeAAAA {
				rr := new(dns.AAAA)
				rr.Hdr = dns.RR_Header{Name: qname, Rrtype: dns.TypeAAAA, Class: dns.ClassINET}
				rr.AAAA = ip.To16()

				answers = append(answers, rr)
			} else if state.QType() == dns.TypeTXT {
				{
					tt := new(dns.TXT)
					tt.Hdr = dns.RR_Header{Name: fmt.Sprintf("mask.%s", qname), Rrtype: dns.TypeTXT, Class: dns.ClassINET}
					tt.Txt = append(tt.Txt, fmt.Sprintf("%s/%d", addr.Addr, addr.Prefix))
					answers = append(answers, tt)
				}
				{
					tt := new(dns.TXT)
					tt.Hdr = dns.RR_Header{Name: fmt.Sprintf("if.%s", qname), Rrtype: dns.TypeTXT, Class: dns.ClassINET}
					tt.Txt = append(tt.Txt, iface.Name)
					answers = append(answers, tt)
				}
			}
		}
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	m.RecursionAvailable = true
	m.Answer = answers

	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (p VirtMachine) Name() string { return "virt" }
