# virt

## Name

*virt* - returns the address of domains running using libvirt (referenced by `name.virt`)

## Description

This plugin connects to libvirtd and will attempt to lookup the domain for any query with the
`.virt` TLD (or other specified), and then return the addresses that have been leased to that
domain.

Note: In order to add a new plugin, an additional step of `make gen` is needed. Therefore,
to build the coredns with demo plugin the following should be used:
```
docker run -it --rm -v $PWD:/v -w /v golang:1.16 sh -c 'make gen && make'
```

## Syntax

~~~ txt
virt [tld] [libvirt socket path] [libvirt connect URI]
~~~

Defaults:
- tld: `virt`
- libvirt socket path: `/run/libvirt/libvirt-sock-ro`
- libvirt connect URI: `qemu:///system`

## Also See

See the [manual](https://coredns.io/manual).
