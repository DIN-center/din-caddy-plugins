# Network-specific Routing Rules

The **din_provider_filter.go** offers an interface for filtering which providers are capable of handling specific requests. The current 
implementation only supports method based routing, but this may be extended in the future.

This routing can be activated by updating the network configuration with:

```
din {
  services {
    ethereum {
        # Method routing configuration
        routed_methods debug_traceBlockByNumber
        providers {
          https://debug.tracer/ {
            priority 0
            methods debug_traceBlockByNumber
          }
          https://other.provider/ {
            priority 0
          }
        }
    }
  }
}
```

In this case any methods other than `debug_traceBlockByNumber` may be routed to either provider, but `debug_traceBlockByNumber` calls will
only be routed to the `debug.tracer` provider.

## Consistency Notes

Normally the DIN router ensures consistency across requests by routing requests with the same Din-Session-Id header to the same provider. This
might mean that most requests for a given Din-Session-Id are routed to `other.provider`, but if they make a `debug_traceBlockByNumber` request,
it must be routed to `debug.tracer` as the only provider capable of handling the request. In this case it's possible that the `debug_traceBlockByNumber`
requests may go to a provider that is ahead or behind other requests in the same session, or perhaps even on a different side of a small reorg.

We have method based routing with more sophisticated consistency guards on the roadmap, but right now it should be clearly communicated to clients that
`routed_methods` may not share the consistency guarantees seen on other methods.
