# DIN Proxy Sidecar Configuration

This configuration supports running the DIN Proxy as an edge service to route to your node infrastructure.

## Caddyfile Config

The DIN Proxy uses a Caddyfile for configuration. This documentation can get you up and running quickly, but you may want to refer to [Caddy's Documentation](https://caddyserver.com/docs/caddyfile) for more detailed configuration options.

A minimal Caddyfile for DIN Edge configuration looks like this:

```
:8000 {
	route /* {
		din_auth {
			whitelist 0x26a52588627FFF0e0EC66f070ee49ecBFECf3BC2 0x2c1DDa3FAA4740ccbf87fEBc2e435c54961a17aD 0x1c8f6be939396592e2c2d29a7b6b0a64d1c71329 0x0eEaFd580145D94534d3a1db5FF42FE68146C251
			secret YOUR_SECRET_KEY_GOES_HERE
		}
		din {
				networks {
				mynetwork {
					providers {
						http://10.0.0.32:3000/API_KEY
						http://10.0.0.34:6000/ALTERNATIVE_API_KEY
					}
					chain_id eip155:0xNetworkChainID
				}
				another-network {
					providers {
						http://10.0.0.54:8545 {
							headers {
								X-API-KEY NETWORK_API_KEY
							}
						}
						http://10.0.0.55:8545
						http://10.0.0.56:8545
					}
					chain_id eip155:0xAnotherNetworkChainID
				}
			}
		}
		# din reverse proxy directive configuration
		# https://caddyserver.com/docs/caddyfile/directives/reverse_proxy
		reverse_proxy {
			# full lb policy name: http.reverse_proxy.upstreams.din_reverse_proxy_policy
			lb_policy din_reverse_proxy_policy

			transport http {
				# tls
				keepalive 10s
			}

			# full selector policy name: http.reverse_proxy.selection_policies.din_reverse_proxy_policy
			dynamic din_reverse_proxy_policy
		}
	}
}
```

Breaking this down by section:

```
din_auth {
	whitelist 0x26a52588627FFF0e0EC66f070ee49ecBFECf3BC2 0x2c1DDa3FAA4740ccbf87fEBc2e435c54961a17aD 0x1c8f6be939396592e2c2d29a7b6b0a64d1c71329 0x0eEaFd580145D94534d3a1db5FF42FE68146C251
	secret YOUR_SECRET_KEY_GOES_HERE
}
```

Here, we configure DIN's authentication middleware. When you make a request to http://localhost:8000/auth, this middleware will handle the request and verify that the request includes a SiWE message signed by one of the whitelisted addresses. The above 4 whitelisted addresses are:

* 0x26a52588627FFF0e0EC66f070ee49ecBFECf3BC2 - The DIN Gateway. This must stay for you to receive traffic.
* 0x2c1DDa3FAA4740ccbf87fEBc2e435c54961a17aD - The DIN Router Developers key. This helps DIN developers with debugging, and we would encourage you to leave it.
* 0x1c8f6be939396592e2c2d29a7b6b0a64d1c71329 and 0x0eEaFd580145D94534d3a1db5FF42FE68146C251 - The DIN Watchers keys. These are needed to pass checks from the watcher, which will soon be necessary to receive traffic.

You may add additional keys for your own purposes, but those 4 should be left in.

The `secret` value is used for HMAC authentication of the session tokens issued by your DIN Proxy. This should be a secret known only to you. If you are running multiple servers, you should use the same key across all DIN Proxy instances to ensure that session keys issued by one instance will be accepted by another.


```
din {
	networks {
		mynetwork {
			providers {
				http://10.0.0.32:3000/API_KEY
				http://10.0.0.34:6000/ALTERNATIVE_API_KEY
			}
			chain_id eip155:0xNetworkChainID
		}
		another-network {
			providers {
				http://10.0.0.54:8545 {
					headers {
						X-API-KEY NETWORK_API_KEY
					}
				}
				http://10.0.0.55:8545
				http://10.0.0.56:8545
			}
			chain_id eip155:0xAnotherNetworkChainID
		}
	}
}
```

This configures the primary DIN middleware.

The proxy can simultaneously support multiple networks. If you support multiple networks for DIN, you can use one set of proxy instances to serve all networks.

You can also list multiple providers for the same network. If you run multiple nodes, you can use the proxy as a load balancer across your nodes.

```
reverse_proxy {
	# full lb policy name: http.reverse_proxy.upstreams.din_reverse_proxy_policy
	lb_policy din_reverse_proxy_policy

	transport http {
		# tls
		keepalive 10s
	}

	# full selector policy name: http.reverse_proxy.selection_policies.din_reverse_proxy_policy
	dynamic din_reverse_proxy_policy
}
```

This configure's Caddy's native reverse proxy middleware with DIN's loadbalancing and selection policies. These should be left alone to take advantage of DIN's healthcheck implementations. Note that in the `transport` section, `tls` is commented out. If your node authenticates over HTTPS, you should uncomment that line.

## Running with Docker

Once you have your Caddyfile saved, you can run it with:

```
docker run -d --restart=always -p 8000:8000 -v /path/to/Caddyfile:/etc/caddy/Caddyfile din-center/din-caddy:latest
```

## Running with Binaries

Caddy binaries with the DIN Plugins included are available [here](https://github.com/DIN-center/din-caddy-plugins/releases).

You can run it with:

```
caddy run --config /etc/caddy/Caddyfile
```

## Testing Your Endpoint

Now you should be able to make requests to http://localhost:8000/mynetwork to access your service - however that access will be gated by the authentication token. For testing, you can use DIN's [siwe-token tool](../../cmd/siwe-token/README.md).