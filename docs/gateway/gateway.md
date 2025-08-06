# DIN Proxy Gateway Configuration

This configuration supports running the DIN Proxy as a gateway to route to other providers.

## Caddyfile Config

The DIN Proxy uses a Caddyfile for configuration.This is an example configuration illustrating a few networks. A more comprehensive sample configuration can be found [here](../../Caddyfile]). These configuration options have secrets redacted. You may want to refer to [Caddy's Documentation](https://caddyserver.com/docs/caddyfile) for more detailed configuration options.

A minimal Caddyfile for DIN Edge configuration looks like this:

```
# Extending Caddy functionality documentation: https://caddyserver.com/docs/extending-caddy

# global admin options: https://caddyserver.com/docs/caddyfile/options#global-options
{
	# admin configuration for metrics
	servers {
		metrics
	}
	# metrics port
	admin :2019
	# logging configuration
	# https://caddyserver.com/docs/caddyfile/directives/log#file
	log {
		output file caddy.log {
			format json
			roll_size 205mb
			roll_keep 2
			roll_keep_for 3d
		}
	}
}
# caddy server main port, not using 8000 as the default port, please update the port in the din middleware to match
:8000 {
	route /* {
		# middleware declaration
		din {
			# the port to listen on, defining here to reference from inside the middleware
			# only needs to be set if the port is not 8000
			port 8000
			siwe-signer {
				secret_file /run/secrets/din-secret-key
			}
			# middleware configurtion data, read by DinMiddleware.UnmarshalCaddyfile()
			networks {
				eth {
					providers {
						https://provider-a.tld/eth {
							auth {
								type siwe
								url https://provider-a.tld/auth
							}
						}
					}
					chain_id 0x1
				}
				holesky {
					providers {
						https://provider-b.tld/api=key {
							priority 0
						}
						https://provider-c.tld/v1/key {
							priority 1
							headers {
								x-api-key key
							}
						}
					}
					chain_id 0x4268
				}
			}
			din_registry {
				registry_enabled false
			}
		}
		# din reverse proxy directive configuration
		# https://caddyserver.com/docs/caddyfile/directives/reverse_proxy
		reverse_proxy {
			lb_policy din_reverse_proxy_policy
			transport http {
				tls
				keepalive 10s
			}
			dynamic din_reverse_proxy_policy
			header_up Host {http.reverse_proxy.upstream.host}
		}
	}
	# Caddy prometheus metrics directive declaration uses http://{HOST}/metrics:2019
	# https://caddyserver.com/docs/metrics#admin-api-metrics
	metrics /metrics {
	}
}

```

Breaking this down by section:

### Metrics configuration

```
# admin configuration for metrics
servers {
	metrics
}
```

Here, we enable [Caddy's Metrics](https://caddyserver.com/docs/metrics) with the default configuration, which servers Prometheus Metrics on the admin API.


### Admin Configuration

Here, we configure [Caddy's Admin API](https://caddyserver.com/docs/api) to run on port 2019. Prometheus Metrics will serve through the Admin API

```
# metrics port
admin :2019
```

### Logging 

Here, we configure [Caddy's logging](https://caddyserver.com/docs/caddyfile/directives/log). Each log file will be 205mb. A maximum of two old log files will be kept for up to 3 days.

```
# logging configuration
# https://caddyserver.com/docs/caddyfile/directives/log#file
log {
	output file caddy.log {
		format json
		roll_size 205mb
		roll_keep 2
		roll_keep_for 3d
	}
}
```

### Authentication Configuration

Here, we configure the signer for [DIN Authentication](../authentication.md). The siwe-signer secret_file should specify the location of an Ethereum private key, which will be used to sign authentication requests for providers using the DIN Authentication Protocol.

```
siwe-signer {
	secret_file /run/secrets/din-secret-key
}
```


### Network Configuration

Here, we show how networks can be configured. First, we'll configure a network with DIN Authentication


```
networks {
	eth {
		providers {
			https://provider-a.tld/eth {
				auth {
					type siwe
					url https://provider-a.tld/auth
				}
			}
		}
		chain_id 0x1
	}
```

Above, we configure the provider URL as the target that serves RPC requests. We also specify the authentication protocol to use the siwe authentication protocol, while authentication requests are sent to https://provider-a.tld/auth. 

Note that the chain ID must also be specified for each network.

Then, we have another network:

```

	holesky {
		providers {
			https://provider-b.tld/api=key {
				priority 0
			}
			https://provider-c.tld/v1/ {
				priority 1
				headers {
					x-api-key key
				}
			}
		}
		chain_id 0x4268
	}
}
```

Here, we have two providers. In this case, we configure provider-b.tld to be priority 0 - the highest priority. Traffic will be routed to provider-b.tld as long as it is passing health checks. Only if provider-b.tld enters unhealthy or warning status will provider-c.tld receive traffic.

Note that provider-c.tld is authenticated with a header, rather than a path. In the provider section of the Caddyfile config, you can specify a set of headers that should be provided with each request for authentication purposes.

Lastly:

```
din_registry {
	registry_enabled false
}
```

Here, we disable DIN Registry based configuration. In the future, we will support adding providers automatically through the on-chain registry, but that is not available at this time.

### Reverse Proxy Configuration

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

This configure's Caddy's native reverse proxy middleware with DIN's loadbalancing and selection policies. These should be left alone to take advantage of DIN's healthcheck implementations. Note that in the `transport` section, `tls` is commented out. If your targets authenticate over HTTPS, you should uncomment that line. Notably, however, Caddy does not support mixing HTTPS targets and HTTP targets in the same config - TLS is enabled or disabled globally.

## Running with Docker

Once you have your Caddyfile saved, you can run it with:

```
docker run -d --restart=always -p 8000:8000 -v /path/to/Caddyfile:/etc/caddy/Caddyfile dincenter/din-caddy:latest
```

## Running with Binaries

Caddy binaries with the DIN Plugins included are available [here](https://github.com/DIN-center/din-caddy-plugins/releases).

You can run it with:

```
caddy run --config /etc/caddy/Caddyfile
```