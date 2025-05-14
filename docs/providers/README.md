# The DIN Proxy for Providers

The DIN Proxy is a build of [Caddy](https://caddyserver.com/) with several customizations built in through Caddy's Plugin system. The customizations help with health checks, provide RPC-aware metrics, and authentication without shared secrets.

Right now, the primary reason providers are being asked to incorporate the DIN Proxy is to support the DIN Authentication system. The DIN Authentication system is [documented in detail here](../authentication.md), but the gist is that it uses the [Sign In With Ethereum](https://docs.login.xyz/) protocol (SiWE) to create authenticated sessions. The Gateway will send a signed request to you, the provider, who will authenticate the request against a whitelist of authorized signers, then issue a session token that can be used to make requests for a period of time. This allows us to list provider URLs in an on-chain registry without having to worry about shared secrets, such as API keys in the URL or headers.

In the future, the DIN Proxy will be involved in processing payments and potentially other elements of the DIN protocol.

There are two main ways to run the DIN proxy:

* If you are running node infrastructure specifically for DIN and want to run the proxy on your existing node infrastructure, you want the [Sidecar Configuration](./sidecar.md).
* If you are serving DIN from shared infrastructure, and want to run the proxy on infrastructure separate from your nodes, you want the [Edge Configuration](./edge.md).