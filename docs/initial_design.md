# DIN Proxy Design

## Intro

The DIN router was initially implemented as a set of plugins for the Traefik proxy, which we found too restrictive to achieve DIN's goals. The relevant limitations were:

- A separate agent was required for monitoring providers, as Traefik’s Yaegi interpreter was not capable of maintaining websocket connections to monitor providers.
- Traefik’s load balancer was positioned after the middleware, leaving no way for the plugin to alter requests after the target provider was known.
- Traefik’s plugin system offered no way for plugins to communicate with each other, so the provider plugin could not pass information to a middleware plugin, for example.
- Traefik’s load balancer offered no way to use different paths for different providers. This could be worked around by load balancing across loopback ports on Traefik and manipulating the request with middleware, but that limited the load balancer’s ability to evaluate target health properly.

## Caddy

Caddy's Plugin system is far more detailed than Traefik's, offering plugin hooks in many places throughout the request process. We implement a set of Caddy plugins using the following hooks.

### http.handlers

This plugin namespace allows us to implement middleware, which can inspect requests, alter them, set request-level variables accessible to other Caddy plugins, inspect responses, and alter them.

It is through this middleware that we implement basic routing capabilities, sophisticated healthchecks, provider authentication, and provider discovery via an on-chain registry.

### http.reverse_proxy.upstreams

This plugin namespace allows us to provide logic for selecting which upstream providers can satisfy a given request. It does not select a specific provider; rather, it returns a complete list of capable providers.

A plugin implementing this namespace handles most of our provider-related logic.

From there, each request is evaluated against the providers available for that service.

### http.reverse_proxy.selection_policies

This plugin namespace is called to choose an upstream provider from the list provided by the http.reverse_proxy.upstreams plugin. It is given access to the request and is able to alter the request itself after the selection is made.

Here, we implement a plugin that alters the request's path after the target selection is made. Additionally, our plugin leverages Caddy's native header-based selection policy to perform the selection, so we don't have to implement our own logic for that.
