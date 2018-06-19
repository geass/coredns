package cnameresolver

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("cnameresolver", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	cnr, err := cnameResolverParse(c)
	if err != nil {
		return plugin.Error("cnameresolver", err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		cnr.Next = next
		return cnr
	})

	return nil
}

func cnameResolverParse(c *caddy.Controller) (CNAMEResolve, error) {
	cnr := CNAMEResolve{}

	i := 0
	for c.Next() {
		if i > 0 {
			return cnr, plugin.ErrOnce
		}
		i++

		cnr.Zones = c.RemainingArgs()
		if len(cnr.Zones) == 0 {
			cnr.Zones = make([]string, len(c.ServerBlockKeys))
			copy(cnr.Zones, c.ServerBlockKeys)
		}
		for i, str := range cnr.Zones {
			cnr.Zones[i] = plugin.Host(str).Normalize()
		}

	}
	return cnr, nil
}
